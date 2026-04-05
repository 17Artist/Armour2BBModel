// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"testing"

	"github.com/17Artist/Armour2BBModel/skin"
)

func makeColor(r, g, b byte) [6]skin.PaintColor {
	c := skin.PaintColor{R: r, G: g, B: b, PaintType: 255}
	return [6]skin.PaintColor{c, c, c, c, c, c}
}

func TestFaceCulling(t *testing.T) {
	cubes := []skin.CubeData{
		{Pos: skin.Vec3i{X: 0, Y: 0, Z: 0}, Type: skin.CubeSolid},
		{Pos: skin.Vec3i{X: 1, Y: 0, Z: 0}, Type: skin.CubeSolid},
	}
	vis := ComputeFaceCulling(cubes)
	if vis[0].Visible[skin.FaceEast] {
		t.Error("cube[0] east should be culled")
	}
	if vis[1].Visible[skin.FaceWest] {
		t.Error("cube[1] west should be culled")
	}
}

func TestConvertHead(t *testing.T) {
	sf := &skin.SkinFile{
		FileVersion: 13,
		SkinType:    "armourers:head",
		Properties:  map[string]string{},
		Parts: []*skin.PartData{{
			PartType: "armourers:head.base",
			ScaleX:   1, ScaleY: 1, ScaleZ: 1,
			Cubes: []skin.CubeData{
				{Pos: skin.Vec3i{X: 0, Y: 0, Z: 0}, Type: skin.CubeSolid, FaceColors: makeColor(255, 0, 0)},
				{Pos: skin.Vec3i{X: 1, Y: 0, Z: 0}, Type: skin.CubeSolid, FaceColors: makeColor(0, 255, 0)},
			},
		}},
	}

	model, err := Convert(sf)
	if err != nil {
		t.Fatal(err)
	}

	// 只应该有 Head 骨骼
	if len(model.Groups) != 1 {
		t.Fatalf("head skin should have 1 group, got %d", len(model.Groups))
	}
	if model.Groups[0].Name != "Head" {
		t.Errorf("expected group name Head, got %s", model.Groups[0].Name)
	}
	// origin 应该是 [0,24,0]
	if model.Groups[0].Origin != [3]float32{0, 24, 0} {
		t.Errorf("expected origin [0,24,0], got %v", model.Groups[0].Origin)
	}
	t.Logf("Elements: %d, Groups: %v", len(model.Elements), model.Groups[0].Name)
}

func TestConvertChest(t *testing.T) {
	sf := &skin.SkinFile{
		FileVersion: 13,
		SkinType:    "armourers:chest",
		Properties:  map[string]string{},
		Parts: []*skin.PartData{
			{PartType: "armourers:chest.base", ScaleX: 1, ScaleY: 1, ScaleZ: 1,
				Cubes: []skin.CubeData{{Pos: skin.Vec3i{}, Type: skin.CubeSolid, FaceColors: makeColor(255, 0, 0)}}},
			{PartType: "armourers:chest.leftArm", ScaleX: 1, ScaleY: 1, ScaleZ: 1,
				Cubes: []skin.CubeData{{Pos: skin.Vec3i{}, Type: skin.CubeSolid, FaceColors: makeColor(0, 255, 0)}}},
			{PartType: "armourers:chest.rightArm", ScaleX: 1, ScaleY: 1, ScaleZ: 1,
				Cubes: []skin.CubeData{{Pos: skin.Vec3i{}, Type: skin.CubeSolid, FaceColors: makeColor(0, 0, 255)}}},
		},
	}

	model, err := Convert(sf)
	if err != nil {
		t.Fatal(err)
	}

	// chest 应该有 Body, RightArm, LeftArm（有 cubes 的骨骼）
	names := map[string]bool{}
	for _, g := range model.Groups {
		names[g.Name] = true
	}
	for _, expected := range []string{"Body", "LeftArm", "RightArm"} {
		if !names[expected] {
			t.Errorf("missing group %s", expected)
		}
	}
	// 不应该有 Head 或 Leg
	for _, bad := range []string{"Head", "RightLeg", "LeftLeg"} {
		if names[bad] {
			t.Errorf("should not have group %s for chest skin", bad)
		}
	}
	t.Logf("Groups: %d, Elements: %d", len(model.Groups), len(model.Elements))
}

func TestConvert8x8x8(t *testing.T) {
	var cubes []skin.CubeData
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			for z := 0; z < 8; z++ {
				cubes = append(cubes, skin.CubeData{
					Pos: skin.Vec3i{X: x, Y: y, Z: z}, Type: skin.CubeSolid,
					FaceColors: makeColor(byte(x*30), byte(y*30), byte(z*30)),
				})
			}
		}
	}

	sf := &skin.SkinFile{
		FileVersion: 13,
		SkinType:    "armourers:head",
		Properties:  map[string]string{},
		Parts:       []*skin.PartData{{PartType: "armourers:head.base", ScaleX: 1, ScaleY: 1, ScaleZ: 1, Cubes: cubes}},
	}

	model, err := Convert(sf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("512 cubes -> %d elements", len(model.Elements))
	if len(model.Elements) > 10 {
		t.Errorf("should merge aggressively, got %d", len(model.Elements))
	}
}
