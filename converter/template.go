// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

// Bone 模板骨骼
type Bone struct {
	Name     string
	Origin   [3]float32 // BBModel 中的旋转中心（来自模板）
	AWOrigin [3]float32 // AW 坐标 (0,0,0) 在 BBModel 中对应的位置（用于坐标转换）
}

// 保存的 AW 几何使用 Y 向下：头部局部 Y=-8..0，胸部 Y=0..12。
// AWOrigin 根据官方 humanoid.json 的关节平移及 part renderOffset 得到；
// 分段 Torso/Hand/Leg2 与完整身体/四肢使用不同原点，不能在同一坐标空间合并。
// 头/胸为 Y=24，Torso 为 18；手臂为 ±5,22，前臂为 ±5,18；
// 大腿为 ±2,12，小腿为 ±2,6；鞋沿用完整腿的 ±2,12。
// 裙为 0,12，翼为 0,24,2。旋转中心 Origin 与几何原点 AWOrigin 是不同概念。

var (
	BoneHead         = Bone{"Head", [3]float32{0, 24, 0}, [3]float32{0, 24, 0}}
	BoneBody         = Bone{"Body", [3]float32{0, 12, 0}, [3]float32{0, 24, 0}}
	BoneTorso        = Bone{"Torso", [3]float32{0, 18, 0}, [3]float32{0, 18, 0}}
	BoneSkirt        = Bone{"Skirt", [3]float32{0, 12, 0}, [3]float32{0, 12, 0}}
	BoneRightArm     = Bone{"RightArm", [3]float32{6, 23, 0}, [3]float32{5, 22, 0}}
	BoneRightForeArm = Bone{"RightForeArm", [3]float32{6, 18, 0}, [3]float32{5, 18, 0}}
	BoneLeftArm      = Bone{"LeftArm", [3]float32{-6, 23, 0}, [3]float32{-5, 22, 0}}
	BoneLeftForeArm  = Bone{"LeftForeArm", [3]float32{-6, 18, 0}, [3]float32{-5, 18, 0}}
	BoneRightLeg     = Bone{"RightLeg", [3]float32{2, 12, 0}, [3]float32{2, 12, 0}}
	BoneRightForeLeg = Bone{"RightForeLeg", [3]float32{2, 6, 0}, [3]float32{2, 6, 0}}
	BoneLeftLeg      = Bone{"LeftLeg", [3]float32{-2, 12, 0}, [3]float32{-2, 12, 0}}
	BoneLeftForeLeg  = Bone{"LeftForeLeg", [3]float32{-2, 6, 0}, [3]float32{-2, 6, 0}}
	BoneLeftFoot     = Bone{"LeftFoot", [3]float32{-2, 6, 0}, [3]float32{-2, 12, 0}}
	BoneRightFoot    = Bone{"RightFoot", [3]float32{2, 6, 0}, [3]float32{2, 12, 0}}
	BoneLeftWing     = Bone{"LeftWing", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
	BoneRightWing    = Bone{"RightWing", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
	BoneLeftPhalanx  = Bone{"LeftPhalanx", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
	BoneRightPhalanx = Bone{"RightPhalanx", [3]float32{0, 12, 2}, [3]float32{0, 24, 2}}
)

// PartToBone AW partType → 模板骨骼
var PartToBone = map[string]*Bone{
	"armourers:head.base":        &BoneHead,
	"armourers:hat.base":         &BoneHead,
	"armourers:chest.base":       &BoneBody,
	"armourers:chest.base2":      &BoneTorso,
	"armourers:chest.leftArm":    &BoneLeftArm,
	"armourers:chest.leftArm2":   &BoneLeftForeArm,
	"armourers:chest.rightArm":   &BoneRightArm,
	"armourers:chest.rightArm2":  &BoneRightForeArm,
	"armourers:legs.leftLeg":     &BoneLeftLeg,
	"armourers:legs.leftLeg2":    &BoneLeftForeLeg,
	"armourers:legs.rightLeg":    &BoneRightLeg,
	"armourers:legs.rightLeg2":   &BoneRightForeLeg,
	"armourers:legs.skirt":       &BoneSkirt,
	"armourers:skirt.base":       &BoneSkirt,
	"armourers:feet.leftFoot":    &BoneLeftFoot,
	"armourers:feet.rightFoot":   &BoneRightFoot,
	"armourers:wings.leftWing":   &BoneLeftWing,
	"armourers:wings.leftWing2":  &BoneLeftPhalanx,
	"armourers:wings.rightWing":  &BoneRightWing,
	"armourers:wings.rightWing2": &BoneRightPhalanx,
}

// Semantic categories come exclusively from the registry type, never the user-editable name.
// Advanced children inherit the attachment of their parent. Unrecognized roots remain explicit.
func resolvePartBone(partType string, parent *Bone) *Bone {
	if bone := PartToBone[partType]; bone != nil {
		return bone
	}
	switch partType {
	case "armourers:part.advanced_part", "armourers:part.locator", "armourers:part.static", "armourers:part.float", "armourers:unknown", "":
		if parent != nil {
			return parent
		}
	}
	names := map[string]string{
		"armourers:sword.base": "Sword", "armourers:pickaxe.base": "Pickaxe", "armourers:axe.base": "Axe",
		"armourers:shovel.base": "Shovel", "armourers:hoe.base": "Hoe", "armourers:shield.base": "Shield",
		"armourers:shield.blocking": "ShieldBlocking", "armourers:trident.base": "Trident", "armourers:trident.throwing": "TridentThrowing",
		"armourers:bow.frame0": "BowFrame0", "armourers:bow.frame1": "BowFrame1", "armourers:bow.frame2": "BowFrame2", "armourers:bow.frame3": "BowFrame3",
		"armourers:bow.arrow": "Arrow", "armourers:bow.base": "BowFrame1", "armourers:arrow.base": "Arrow",
		"armourers:fishing.rod": "FishingRod", "armourers:fishing.rod1": "FishingRodCast", "armourers:fishing.hook": "FishingHook",
		"armourers:backpack.base": "Backpack", "armourers:item.base": "Item", "armourers:block.base": "Block", "armourers:block.multiblock": "Multiblock",
		"armourers:boat.base": "Boat", "armourers:boat.leftPaddle": "LeftPaddle", "armourers:boat.rightPaddle": "RightPaddle", "armourers:minecart.base": "Minecart",
	}
	name := names[partType]
	if name == "" {
		name = partType
	}
	if name == "" {
		name = "Part"
	}
	return &Bone{Name: name}
}

// SkinTypeBones 是类型的参考骨骼列表。转换使用实际注册部件，不以该名单过滤几何。
var SkinTypeBones = map[string][]*Bone{
	"armourers:head":  {&BoneHead},
	"armourers:chest": {&BoneBody, &BoneTorso, &BoneRightArm, &BoneRightForeArm, &BoneLeftArm, &BoneLeftForeArm},
	"armourers:legs":  {&BoneRightLeg, &BoneLeftLeg, &BoneRightForeLeg, &BoneLeftForeLeg, &BoneSkirt},
	"armourers:feet":  {&BoneRightFoot, &BoneLeftFoot},
	"armourers:wings": {&BoneLeftWing, &BoneRightWing, &BoneLeftPhalanx, &BoneRightPhalanx},
	"armourers:outfit": {
		&BoneHead, &BoneBody, &BoneTorso, &BoneSkirt,
		&BoneRightArm, &BoneRightForeArm, &BoneLeftArm, &BoneLeftForeArm,
		&BoneRightLeg, &BoneRightForeLeg, &BoneLeftLeg, &BoneLeftForeLeg,
		&BoneLeftFoot, &BoneRightFoot,
		&BoneLeftWing, &BoneRightWing, &BoneLeftPhalanx, &BoneRightPhalanx,
	},
}
