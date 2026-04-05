// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/17Artist/Armour2BBModel/bbmodel"
	"github.com/17Artist/Armour2BBModel/skin"
)

type WingAnimParams struct {
	MinAngle  float64
	MaxAngle  float64
	IdleSpeed float64
	MoveType  string
}

func DefaultWingParams() WingAnimParams {
	return WingAnimParams{MinAngle: 0, MaxAngle: 75, IdleSpeed: 6000, MoveType: "EASE"}
}

func ParseWingParams(props map[string]string) WingAnimParams {
	p := DefaultWingParams()
	if v, ok := props["wingsMaxAngle"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			p.MaxAngle = f
		}
	}
	if v, ok := props["wingsMinAngle"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			p.MinAngle = f
		}
	}
	if v, ok := props["wingsIdleSpeed"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			p.IdleSpeed = f
		}
	}
	if v, ok := props["wingsMovmentType"]; ok {
		p.MoveType = v
	}
	return p
}

func fmtAngle(v float64) string {
	if v == 0 {
		return "0"
	}
	return fmt.Sprintf("%.1f", v)
}

// markerToBBOrigin 将 AW marker 坐标转为 BBModel 绝对坐标（方块中心）
// AW marker (x,y,z) 指向一个方块位置，方块在 BBModel 中的中心是：
//
//	X: -(x+0.5) + awOrigin.X  (X翻转，方块中心在 -x-0.5)
//	Y: -(y+0.5) + awOrigin.Y  (Y翻转)
//	Z: z+0.5 + awOrigin.Z     (Z不翻转)
func markerToBBOrigin(marker skin.Marker, awOrigin [3]float32) [3]float32 {
	return [3]float32{
		-float32(marker.X) - 0.5 + awOrigin[0],
		-float32(marker.Y) - 0.5 + awOrigin[1],
		float32(marker.Z) + 0.5 + awOrigin[2],
	}
}

// GenerateWingAnimation 生成翅膀 idle 动画
// 每个翅膀的枢轴点由各自的 marker 位置决定
func GenerateWingAnimation(model *bbmodel.Model, sf *skin.SkinFile, groupUUIDs map[string]string) {
	params := ParseWingParams(sf.Properties)
	periodSec := math.Round(params.IdleSpeed/1000.0*100) / 100

	interpolation := "linear"
	if params.MoveType == "EASE" {
		interpolation = "catmullrom"
	}

	type wingData struct {
		boneName string
		bone     *Bone
		marker   skin.Marker
		isLeft   bool
	}
	var wings []wingData

	for _, part := range sf.Parts {
		if len(part.Markers) == 0 {
			continue
		}
		bone := PartToBone[part.PartType]
		if bone == nil {
			continue
		}
		isLeft := strings.Contains(part.PartType, "left") || strings.Contains(part.PartType, "Left")
		switch part.PartType {
		case "armourers:wings.leftWing", "armourers:wings.leftWing2",
			"armourers:wings.rightWing", "armourers:wings.rightWing2":
			wings = append(wings, wingData{
				boneName: bone.Name,
				bone:     bone,
				marker:   part.Markers[0],
				isLeft:   isLeft,
			})
		}
	}

	if len(wings) == 0 {
		return
	}

	// 将翅膀骨骼 origin 更新为 marker 位置
	for _, w := range wings {
		origin := markerToBBOrigin(w.marker, w.bone.AWOrigin)
		for i := range model.Groups {
			if model.Groups[i].Name == w.boneName {
				model.Groups[i].Origin = origin
			}
		}
		prefix := w.boneName + "_"
		for i := range model.Elements {
			if len(model.Elements[i].Name) >= len(prefix) && model.Elements[i].Name[:len(prefix)] == prefix {
				model.Elements[i].Origin = origin
			}
		}
	}

	animators := map[string]bbmodel.Animator{}

	makeKFs := func(minA, maxA float64) []bbmodel.Keyframe {
		return []bbmodel.Keyframe{
			{Channel: "rotation", DataPoints: []bbmodel.DataPoint{{X: "0", Y: fmtAngle(minA), Z: "0"}},
				UUID: bbmodel.NewUUID(), Time: 0, Color: -1, Interpolation: interpolation},
			{Channel: "rotation", DataPoints: []bbmodel.DataPoint{{X: "0", Y: fmtAngle(maxA), Z: "0"}},
				UUID: bbmodel.NewUUID(), Time: periodSec / 2, Color: -1, Interpolation: interpolation},
			{Channel: "rotation", DataPoints: []bbmodel.DataPoint{{X: "0", Y: fmtAngle(minA), Z: "0"}},
				UUID: bbmodel.NewUUID(), Time: periodSec, Color: -1, Interpolation: interpolation},
		}
	}

	for _, w := range wings {
		uuid, ok := groupUUIDs[w.boneName]
		if !ok {
			continue
		}
		// V5 rotation Y 取反
		// 右翼向外展开 = 实际 +Y → bbmodel -Y
		// 左翼向外展开 = 实际 -Y → bbmodel +Y
		if w.isLeft {
			animators[uuid] = bbmodel.Animator{
				Name: w.boneName, Type: "bone",
				Keyframes: makeKFs(params.MinAngle, params.MaxAngle),
			}
		} else {
			animators[uuid] = bbmodel.Animator{
				Name: w.boneName, Type: "bone",
				Keyframes: makeKFs(-params.MinAngle, -params.MaxAngle),
			}
		}
	}

	if len(animators) == 0 {
		return
	}

	model.Animations = append(model.Animations, bbmodel.Animation{
		UUID: bbmodel.NewUUID(), Name: "idle", Loop: "loop",
		Length: periodSec, Snapping: 24, Animators: animators,
	})
}
