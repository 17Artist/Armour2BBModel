// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"fmt"
	"math"
	"strconv"

	"github.com/17Artist/Armour2BBModel/bbmodel"
	"github.com/17Artist/Armour2BBModel/skin"
)

type WingAnimParams struct {
	MinAngle, MaxAngle, IdleSpeed float64
	MoveType                      string
}

func DefaultWingParams() WingAnimParams {
	return WingAnimParams{MinAngle: 0, MaxAngle: 75, IdleSpeed: 6000, MoveType: "EASE"}
}

func ParseWingParams(props map[string]string) WingAnimParams {
	p := DefaultWingParams()
	for _, entry := range []struct {
		key string
		dst *float64
	}{
		{"wingsMinAngle", &p.MinAngle}, {"wingsMaxAngle", &p.MaxAngle}, {"wingsIdleSpeed", &p.IdleSpeed},
	} {
		if v, ok := props[entry.key]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
				*entry.dst = f
			}
		}
	}
	if v, ok := props["wingsMovmentType"]; ok {
		p.MoveType = v
	}
	return p
}

// A wing is an attachment mechanism, not necessarily the visible anatomy of a costume.
// Bind by the source part itself: outfits can contain many parts of the same wing type.
type wingBinding struct {
	part            *skin.PartData
	groupUUID, name string
	params          WingAnimParams
	axis, worldAxis [3]float64
	baseAngle       float64
	parent          affine
	origin          [3]float32
}

func wingForPart(p *skin.PartData, sf *skin.SkinFile, parent affine) *wingBinding {
	left := false
	switch p.PartType {
	case "armourers:wings.leftWing":
		left = true
	case "armourers:wings.rightWing":
	default:
		return nil
	}
	if len(p.Markers) == 0 {
		return nil
	}
	marker := p.Markers[0]
	// SkinMarker.direction = OpenDirection(meta - 1); NONE means no rotation.
	var axis [3]float64
	switch marker.Meta {
	case 1:
		axis = [3]float64{0, -1, 0} // DOWN
	case 2:
		axis = [3]float64{0, 1, 0} // UP
	case 3:
		axis = [3]float64{0, 0, 1} // NORTH
	case 4:
		axis = [3]float64{0, 0, -1} // SOUTH
	case 5:
		axis = [3]float64{-1, 0, 0} // WEST
	case 6:
		axis = [3]float64{1, 0, 0} // EAST
	default:
		return nil
	}
	if !left {
		for i := range axis {
			axis[i] = -axis[i]
		}
	}
	props := p.Properties
	// Programmatic callers may omit the bound part map. Parsed files always have a map.
	if props == nil && sf.SkinType == "armourers:wings" {
		props = sf.Properties
	}
	if len(props) == 0 {
		return nil
	}
	w := &wingBinding{part: p, params: ParseWingParams(props), axis: axis, parent: parent}
	w.baseAngle = w.angle(0)
	// The saved marker is in the parent coordinate frame, before the part transform.
	point := parent.point(float64(marker.X)+.5, float64(marker.Y)+.5, float64(marker.Z)+.5)
	bone := PartToBone[p.PartType]
	w.origin = [3]float32{-float32(point[0]) + bone.AWOrigin[0], -float32(point[1]) + bone.AWOrigin[1], float32(point[2]) + bone.AWOrigin[2]}
	signs := [3]float64{-1, -1, 1}
	length := 0.0
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			w.worldAxis[row] += signs[row] * parent[row*4+col] * axis[col]
		}
		length += w.worldAxis[row] * w.worldAxis[row]
	}
	length = math.Sqrt(length)
	if length > 0 {
		for i := range w.worldAxis {
			w.worldAxis[i] /= length
		}
	}
	return w
}

func (w *wingBinding) period() float64 { return math.Max(w.params.IdleSpeed/1000, .1) }

// This matches WingPartTransform.rotationDegrees, including LINEAR's deliberate sawtooth.
func (w *wingBinding) angle(phase float64) float64 {
	rangeAngle := w.params.MaxAngle - w.params.MinAngle
	if w.params.MoveType == "LINEAR" {
		return rangeAngle * phase
	}
	return -w.params.MinAngle - rangeAngle*(math.Sin(phase*2*math.Pi)+1)/2
}

func axisRotation(axis [3]float64, degrees float64) affine {
	x, y, z := axis[0], axis[1], axis[2]
	c, s := math.Cos(degrees*math.Pi/180), math.Sin(degrees*math.Pi/180)
	t := 1 - c
	return affine{x*x*t + c, x*y*t - z*s, x*z*t + y*s, 0, y*x*t + z*s, y*y*t + c, y*z*t - x*s, 0, z*x*t - y*s, z*y*t + x*s, z*z*t + c, 0, 0, 0, 0, 1}
}

func (w *wingBinding) localMatrix() affine {
	m := w.part.Markers[0]
	x, y, z := float64(m.X)+.5, float64(m.Y)+.5, float64(m.Z)+.5
	return multiply(multiply(translation(x, y, z), axisRotation(w.axis, w.baseAngle)), translation(-x, -y, -z))
}

func appendWingAnimations(model *bbmodel.Model, wings []*wingBinding) {
	// Separate source equipments can have different periods. Keep independent animations;
	// constant-angle parts already have their exact pose baked, and need no idle animation.
	byKey := map[string]int{}
	for _, w := range wings {
		if w.params.MinAngle == w.params.MaxAngle {
			continue
		}
		key := fmt.Sprintf("%v", w.period())
		name := "idle"
		if w.part.EquipmentIndex != nil {
			key = fmt.Sprintf("%d:%s", *w.part.EquipmentIndex, key)
			name = fmt.Sprintf("idle_equipment_%d", *w.part.EquipmentIndex+1)
		}
		index, ok := byKey[key]
		if !ok {
			index = len(model.Animations)
			byKey[key] = index
			model.Animations = append(model.Animations, bbmodel.Animation{UUID: bbmodel.NewUUID(), Name: name, Loop: "loop", Length: w.period(), Snapping: 24, Animators: map[string]bbmodel.Animator{}})
		}
		steps := 32 // sampled sine; LINEAR needs only its endpoints and the loop discontinuity
		if w.params.MoveType == "LINEAR" {
			steps = 1
		}
		frames := make([]bbmodel.Keyframe, 0, steps+1)
		for step := 0; step <= steps; step++ {
			phase := float64(step) / float64(steps)
			delta := w.angle(phase) - w.baseAngle
			r := axisRotation(w.worldAxis, delta)
			_, _, _, rotation, err := mapTransformedBox(MergedBox{MaxX: 1, MaxY: 1, MaxZ: 1}, r, [3]float32{})
			if err != nil {
				continue
			}
			// mapTransformedBox changes the AW basis; r already uses the BB basis.
			rotation[0], rotation[1] = -rotation[0], -rotation[1]
			// Do not wrap a full turn into a short Euler angle on a principal axis.
			for axis, value := range w.worldAxis {
				if math.Abs(math.Abs(value)-1) < 1e-6 {
					rotation = [3]float32{}
					rotation[axis] = float32(value * delta)
					break
				}
			}
			fmtValue := func(v float32) string {
				if math.Abs(float64(v)) < 1e-5 {
					return "0"
				}
				return strconv.FormatFloat(float64(v), 'f', 5, 32)
			}
			frames = append(frames, bbmodel.Keyframe{Channel: "rotation", DataPoints: []bbmodel.DataPoint{{X: fmtValue(rotation[0]), Y: fmtValue(rotation[1]), Z: fmtValue(rotation[2])}}, UUID: bbmodel.NewUUID(), Time: phase * w.period(), Color: -1, Interpolation: "linear"})
		}
		model.Animations[index].Animators[w.groupUUID] = bbmodel.Animator{Name: w.name, Type: "bone", Keyframes: frames}
	}
}
