// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

// Bone 模板骨骼
type Bone struct {
	Name     string
	Origin   [3]float32 // BBModel 中的旋转中心（来自模板）
	AWOrigin [3]float32 // AW 坐标 (0,0,0) 在 BBModel 中对应的位置（用于坐标转换）
}

// AWOrigin 的推导方法：
//   AW guideSpace 的起点在 AW 坐标系中的位置 → 对应原版模型在 BBModel 中的位置
//   从而推算 AW (0,0,0) 在 BBModel 中的位置
//
// Head:     guide(-4,0,-4) → BB(-4,24,-4)  → AW(0,0,0) = BB(0,24,0)
// Chest:    guide(-4,-12,-2) → BB(-4,12,-2) → AW(0,0,0) = BB(0,24,0)
// LeftArm:  guide(-3,-10,-2) → BB(-8,12,-2), renderOffset(5,2,0) → AW(0,0,0) = BB(-5,22,0)
// RightArm: guide(-1,-10,-2) → BB(4,12,-2), renderOffset(-5,2,0) → AW(0,0,0) = BB(5,22,0)
// LeftLeg:  guide(-2,-12,-2) → BB(-4,0,-2), renderOffset(2,12,0) → AW(0,0,0) = BB(-2,12,0)
// RightLeg: guide(-2,-12,-2) → BB(0,0,-2), renderOffset(-2,12,0) → AW(0,0,0) = BB(2,12,0)
// LeftFoot: guide(-2,-12,-2) → BB(-4,0,-2), renderOffset(2,12,0) → AW(0,0,0) = BB(-2,12,0)
// RightFoot:guide(-2,-12,-2) → BB(0,0,-2), renderOffset(-2,12,0) → AW(0,0,0) = BB(2,12,0)
// Wings:    renderOffset(0,0,2) → AW(0,0,0) = BB(0,24,2) (body 顶部偏后)

var (
	BoneHead         = Bone{"Head", [3]float32{0, 24, 0}, [3]float32{0, 24, 0}}
	BoneBody         = Bone{"Body", [3]float32{0, 12, 0}, [3]float32{0, 24, 0}}
	BoneRightArm     = Bone{"RightArm", [3]float32{6, 22, 0}, [3]float32{5, 22, 0}}
	BoneRightForeArm = Bone{"RightForeArm", [3]float32{6, 18, 2}, [3]float32{5, 22, 0}}
	BoneLeftArm      = Bone{"LeftArm", [3]float32{-5, 22, 0}, [3]float32{-5, 22, 0}}
	BoneLeftForeArm  = Bone{"LeftForeArm", [3]float32{-4, 18, 2}, [3]float32{-5, 22, 0}}
	BoneRightLeg     = Bone{"RightLeg", [3]float32{2, 12, 0}, [3]float32{2, 12, 0}}
	BoneRightForeLeg = Bone{"RightForeLeg", [3]float32{2, 6, -2}, [3]float32{2, 12, 0}}
	BoneLeftLeg      = Bone{"LeftLeg", [3]float32{-2, 12, 0}, [3]float32{-2, 12, 0}}
	BoneLeftForeLeg  = Bone{"LeftForeLeg", [3]float32{-2, 6, -2}, [3]float32{-2, 12, 0}}
	BoneLeftWing     = Bone{"LeftWing", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
	BoneRightWing    = Bone{"RightWing", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
)

// PartToBone AW partType → 模板骨骼
var PartToBone = map[string]*Bone{
	"armourers:head.base":        &BoneHead,
	"armourers:hat.base":         &BoneHead,
	"armourers:chest.base":       &BoneBody,
	"armourers:chest.base2":      &BoneBody,
	"armourers:chest.leftArm":    &BoneLeftArm,
	"armourers:chest.leftArm2":   &BoneLeftForeArm,
	"armourers:chest.rightArm":   &BoneRightArm,
	"armourers:chest.rightArm2":  &BoneRightForeArm,
	"armourers:legs.leftLeg":     &BoneLeftLeg,
	"armourers:legs.leftLeg2":    &BoneLeftForeLeg,
	"armourers:legs.rightLeg":    &BoneRightLeg,
	"armourers:legs.rightLeg2":   &BoneRightForeLeg,
	"armourers:legs.skirt":       &BoneBody,
	"armourers:feet.leftFoot":    &BoneLeftForeLeg,
	"armourers:feet.rightFoot":   &BoneRightForeLeg,
	"armourers:wings.leftWing":   &BoneLeftWing,
	"armourers:wings.leftWing2":  &BoneLeftWing,
	"armourers:wings.rightWing":  &BoneRightWing,
	"armourers:wings.rightWing2": &BoneRightWing,
}

// SkinTypeBones AW skinType → 该类型需要输出的骨骼列表
var SkinTypeBones = map[string][]*Bone{
	"armourers:head":  {&BoneHead},
	"armourers:chest": {&BoneBody, &BoneRightArm, &BoneRightForeArm, &BoneLeftArm, &BoneLeftForeArm},
	"armourers:legs":  {&BoneRightLeg, &BoneLeftLeg},
	"armourers:feet":  {&BoneRightForeLeg, &BoneLeftForeLeg},
	"armourers:wings": {&BoneLeftWing, &BoneRightWing},
	"armourers:outfit": {
		&BoneHead, &BoneBody,
		&BoneRightArm, &BoneRightForeArm, &BoneLeftArm, &BoneLeftForeArm,
		&BoneRightLeg, &BoneRightForeLeg, &BoneLeftLeg, &BoneLeftForeLeg,
		&BoneLeftWing, &BoneRightWing,
	},
}
