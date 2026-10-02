package converter

import (
	"fmt"

	"github.com/17Artist/Armour2BBModel/bbmodel"
	"github.com/17Artist/Armour2BBModel/skin"
)

// ArcartX filters model roots by exact, case-sensitive host bone names. Source
// attachment metadata is deliberately independent from these five target slots.
var targetRootOrder = []string{"Head", "Body", "RightArm", "RightForeArm", "LeftArm", "LeftForeArm", "RightLeg", "LeftLeg", "RightForeLeg", "LeftForeLeg", "Decoration"}

func targetRootFor(source string) string {
	switch source {
	case "Head", "Body", "RightArm", "RightForeArm", "LeftArm", "LeftForeArm", "RightLeg", "LeftLeg":
		return source
	case "Torso":
		return "Body"
	case "LeftForeLeg", "Skirt":
		return "LeftLeg"
	case "RightForeLeg":
		return "RightLeg"
	case "LeftFoot":
		return "LeftForeLeg"
	case "RightFoot":
		return "RightForeLeg"
	default:
		return "Decoration"
	}
}

func slotForTarget(target string) string {
	switch target {
	case "Head":
		return "HEAD"
	case "Body", "RightArm", "RightForeArm", "LeftArm", "LeftForeArm":
		return "BODY"
	case "RightLeg", "LeftLeg":
		return "LEGS"
	case "RightForeLeg", "LeftForeLeg":
		return "FEET"
	default:
		return "DECORATION"
	}
}

func newTargetGroup(target string) bbmodel.Group {
	var origin [3]float32
	switch target {
	case "Head":
		origin = BoneHead.Origin
	case "Body":
		origin = BoneBody.Origin
	case "LeftArm":
		origin = BoneLeftArm.Origin
	case "RightArm":
		origin = BoneRightArm.Origin
	case "LeftForeArm":
		origin = BoneLeftForeArm.Origin
	case "RightForeArm":
		origin = BoneRightForeArm.Origin
	case "LeftLeg":
		origin = BoneLeftLeg.Origin
	case "RightLeg":
		origin = BoneRightLeg.Origin
	case "LeftForeLeg":
		origin = BoneLeftForeLeg.Origin
	case "RightForeLeg":
		origin = BoneRightForeLeg.Origin
	}
	return bbmodel.Group{UUID: bbmodel.NewUUID(), Name: target, Attachment: target, TargetBone: target, CostumeSlot: slotForTarget(target), JointOnly: true, Origin: origin, Export: true, Shade: true, Visibility: true, IsOpen: true, Children: []any{}}
}

func fragmentWork(w *partWork, target string, cubes []skin.CubeData) *partWork {
	f := *w
	f.target, f.cubes, f.boxes, f.regions, f.children = target, cubes, nil, nil, nil
	f.group = w.group
	f.group.UUID = bbmodel.NewUUID()
	f.group.Name = fmt.Sprintf("%s_%s", w.group.Name, target)
	f.group.TargetBone, f.group.CostumeSlot, f.group.Children = target, slotForTarget(target), []any{}
	return &f
}

func sourceWorldCenter(w *partWork, cube skin.CubeData) [3]float64 {
	p := w.matrix.point(float64(cube.Pos.X)+.5, float64(cube.Pos.Y)+.5, float64(cube.Pos.Z)+.5)
	return [3]float64{-p[0] + float64(w.bone.AWOrigin[0]), -p[1] + float64(w.bone.AWOrigin[1]), p[2] + float64(w.bone.AWOrigin[2])}
}

func classifyTargetWorks(works []*partWork) []*partWork {
	// A skirt is source geometry, not an additional equipment slot. Split its
	// source voxels on the world sagittal plane before merging, so neither side
	// is duplicated and each half can follow its corresponding target leg.
	var result []*partWork
	var visit func(*partWork) []*partWork
	visit = func(w *partWork) []*partWork {
		var same []*partWork
		for _, child := range w.children {
			for _, fragment := range visit(child) {
				if fragment.target == w.target {
					same = append(same, fragment)
				} else {
					result = append(result, fragment)
				}
			}
		}
		w.children = same
		if w.bone.Name == "Skirt" && len(w.children) == 0 && !w.moving {
			var fragments []*partWork
			bins := map[string][]skin.CubeData{}
			for _, cube := range w.cubes {
				target := "LeftLeg"
				if sourceWorldCenter(w, cube)[0] >= 0 {
					target = "RightLeg"
				}
				bins[target] = append(bins[target], cube)
			}
			for _, target := range []string{"LeftLeg", "RightLeg"} {
				if len(bins[target]) > 0 {
					fragments = append(fragments, fragmentWork(w, target, bins[target]))
				}
			}
			if len(w.cubes) == 0 {
				fragments = append(fragments, w)
			}
			return fragments
		}
		return []*partWork{w}
	}
	for _, w := range works {
		result = append(result, visit(w)...)
	}
	return result
}
