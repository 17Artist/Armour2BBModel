// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"github.com/17Artist/Armour2BBModel/skin"
)

// FaceVisibility 记录每个 cube 每个面是否可见
type FaceVisibility struct {
	Visible [6]bool
}

// ComputeFaceCulling 在 AW 原始坐标空间计算面可见性。
// 面方向偏移在 AW 空间中是正确的，不受后续坐标翻转影响。
func ComputeFaceCulling(cubes []skin.CubeData) []FaceVisibility {
	result := make([]FaceVisibility, len(cubes))
	for i := range result {
		for f := 0; f < 6; f++ {
			result[i].Visible[f] = true
		}
	}

	if len(cubes) <= 1 {
		return result
	}

	posMap := make(map[skin.Vec3i]int, len(cubes))
	for i := range cubes {
		posMap[cubes[i].Pos] = i
	}

	for i := range cubes {
		c := &cubes[i]
		for face := 0; face < 6; face++ {
			off := skin.FaceOffset(face)
			nPos := skin.Vec3i{X: c.Pos.X + off.X, Y: c.Pos.Y + off.Y, Z: c.Pos.Z + off.Z}
			ni, exists := posMap[nPos]
			if !exists {
				continue
			}
			if !c.Type.IsGlass() && !cubes[ni].Type.IsGlass() {
				result[i].Visible[face] = false
			}
		}
	}

	return result
}
