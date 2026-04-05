// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"sort"

	"github.com/17Artist/Armour2BBModel/skin"
)

type MergedBox struct {
	MinX, MinY, MinZ int
	MaxX, MaxY, MaxZ int
	CubeType         skin.CubeType
	Faces            [6]MergedFace
}

type MergedFace struct {
	Visible bool
	Colors  [][]skin.PaintColor
	Width   int
	Height  int
}

func (b *MergedBox) SizeX() int { return b.MaxX - b.MinX }
func (b *MergedBox) SizeY() int { return b.MaxY - b.MinY }
func (b *MergedBox) SizeZ() int { return b.MaxZ - b.MinZ }

type mergeEntry struct {
	idx  int
	cube skin.CubeData
}

// GreedyMerge 尝试 3轴×2方向=6 种切片策略，取 box 数最少的
func GreedyMerge(cubes []skin.CubeData, vis []FaceVisibility) []MergedBox {
	if len(cubes) == 0 {
		return nil
	}

	var best []MergedBox
	for _, axis := range []int{0, 1, 2} {
		for _, swap := range []bool{false, true} {
			r := sliceMerge(cubes, axis, swap)
			if best == nil || len(r) < len(best) {
				best = r
			}
		}
	}

	posMap := make(map[skin.Vec3i]*mergeEntry, len(cubes))
	for i := range cubes {
		posMap[cubes[i].Pos] = &mergeEntry{idx: i, cube: cubes[i]}
	}
	for i := range best {
		buildMergedFaces(&best[i], posMap, vis)
	}
	return best
}

// sliceMerge 按切片轴分层，每层行扫描贪心合并，再沿切片轴扩展
func sliceMerge(cubes []skin.CubeData, sliceAxis int, swap bool) []MergedBox {
	posIdx := make(map[skin.Vec3i]int, len(cubes))
	for i := range cubes {
		posIdx[cubes[i].Pos] = i
	}
	used := make([]bool, len(cubes))

	getAxis := func(p skin.Vec3i) (s, a, b int) {
		switch sliceAxis {
		case 0:
			s, a, b = p.X, p.Y, p.Z
		case 1:
			s, a, b = p.Y, p.X, p.Z
		default:
			s, a, b = p.Z, p.X, p.Y
		}
		if swap {
			a, b = b, a
		}
		return
	}
	makeP := func(s, a, b int) skin.Vec3i {
		ra, rb := a, b
		if swap {
			ra, rb = b, a
		}
		switch sliceAxis {
		case 0:
			return skin.Vec3i{X: s, Y: ra, Z: rb}
		case 1:
			return skin.Vec3i{X: ra, Y: s, Z: rb}
		default:
			return skin.Vec3i{X: ra, Y: rb, Z: s}
		}
	}
	canUse := func(s, a, b int, ct skin.CubeType) bool {
		idx, ok := posIdx[makeP(s, a, b)]
		return ok && !used[idx] && cubes[idx].Type == ct
	}
	markUsed := func(s, a, b int) {
		if idx, ok := posIdx[makeP(s, a, b)]; ok {
			used[idx] = true
		}
	}

	// 分层
	slices := map[int][]int{}
	for i := range cubes {
		sv, _, _ := getAxis(cubes[i].Pos)
		slices[sv] = append(slices[sv], i)
	}
	sliceVals := make([]int, 0, len(slices))
	for k := range slices {
		sliceVals = append(sliceVals, k)
	}
	sort.Ints(sliceVals)

	var result []MergedBox

	for _, sv := range sliceVals {
		indices := slices[sv]

		// 找到该层的 A/B 范围
		minA, maxA, minB, maxB := 1<<30, -(1 << 30), 1<<30, -(1 << 30)
		for _, ci := range indices {
			_, a, b := getAxis(cubes[ci].Pos)
			if a < minA {
				minA = a
			}
			if a > maxA {
				maxA = a
			}
			if b < minB {
				minB = b
			}
			if b > maxB {
				maxB = b
			}
		}
		wa, wb := maxA-minA+1, maxB-minB+1

		// 构建 2D 网格（按类型分别处理）
		typeSet := map[skin.CubeType]bool{}
		for _, ci := range indices {
			typeSet[cubes[ci].Type] = true
		}

		for ct := range typeSet {
			// 构建该类型的 2D 占用网格
			grid := make([][]bool, wb)
			for j := range grid {
				grid[j] = make([]bool, wa)
			}
			for _, ci := range indices {
				if used[ci] || cubes[ci].Type != ct {
					continue
				}
				_, a, b := getAxis(cubes[ci].Pos)
				grid[b-minB][a-minA] = true
			}

			// 行扫描贪心合并
			for bIdx := 0; bIdx < wb; bIdx++ {
				for aIdx := 0; aIdx < wa; aIdx++ {
					if !grid[bIdx][aIdx] {
						continue
					}
					a0, b0 := aIdx+minA, bIdx+minB
					// +A
					aEnd := a0 + 1
					for aEnd-minA < wa && grid[bIdx][aEnd-minA] {
						aEnd++
					}
					// +B
					bEnd := b0 + 1
					for bEnd-minB < wb {
						rowOk := true
						for a := a0; a < aEnd; a++ {
							if !grid[bEnd-minB][a-minA] {
								rowOk = false
								break
							}
						}
						if !rowOk {
							break
						}
						bEnd++
					}
					result = append(result, emitBox(a0, b0, aEnd, bEnd,
						sv, ct, grid, minA, minB, sliceAxis, swap, canUse, markUsed))
				}
			}
		}
	}

	return result
}

// emitBox 提取一个矩形区域，沿切片轴扩展，标记已用，返回 MergedBox
func emitBox(a0, b0, aEnd, bEnd, sv int, ct skin.CubeType,
	grid [][]bool, minA, minB int, sliceAxis int, swap bool,
	canUse func(int, int, int, skin.CubeType) bool,
	markUsed func(int, int, int)) MergedBox {

	// 沿切片轴扩展
	sEnd := sv + 1
	for {
		allOk := true
		for b := b0; b < bEnd && allOk; b++ {
			for a := a0; a < aEnd; a++ {
				if !canUse(sEnd, a, b, ct) {
					allOk = false
					break
				}
			}
		}
		if !allOk {
			break
		}
		for b := b0; b < bEnd; b++ {
			for a := a0; a < aEnd; a++ {
				markUsed(sEnd, a, b)
			}
		}
		sEnd++
	}

	// 标记当前层
	for b := b0; b < bEnd; b++ {
		for a := a0; a < aEnd; a++ {
			markUsed(sv, a, b)
			if b-minB >= 0 && b-minB < len(grid) && a-minA >= 0 && a-minA < len(grid[b-minB]) {
				grid[b-minB][a-minA] = false
			}
		}
	}

	ra0, rb0, raEnd, rbEnd := a0, b0, aEnd, bEnd
	if swap {
		ra0, rb0 = b0, a0
		raEnd, rbEnd = bEnd, aEnd
	}

	box := MergedBox{CubeType: ct}
	switch sliceAxis {
	case 0:
		box.MinX, box.MinY, box.MinZ = sv, ra0, rb0
		box.MaxX, box.MaxY, box.MaxZ = sEnd, raEnd, rbEnd
	case 1:
		box.MinX, box.MinY, box.MinZ = ra0, sv, rb0
		box.MaxX, box.MaxY, box.MaxZ = raEnd, sEnd, rbEnd
	default:
		box.MinX, box.MinY, box.MinZ = ra0, rb0, sv
		box.MaxX, box.MaxY, box.MaxZ = raEnd, rbEnd, sEnd
	}
	return box
}

func buildMergedFaces(box *MergedBox, posMap map[skin.Vec3i]*mergeEntry, vis []FaceVisibility) {
	sx, sy, sz := box.SizeX(), box.SizeY(), box.SizeZ()

	box.Faces[skin.FaceDown] = buildFace(skin.FaceDown, sx, sz, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY, Z: box.MinZ + v}
	}, posMap, vis)
	box.Faces[skin.FaceUp] = buildFace(skin.FaceUp, sx, sz, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MinX + u, Y: box.MaxY - 1, Z: box.MinZ + v}
	}, posMap, vis)
	box.Faces[skin.FaceNorth] = buildFace(skin.FaceNorth, sx, sy, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY + v, Z: box.MinZ}
	}, posMap, vis)
	box.Faces[skin.FaceSouth] = buildFace(skin.FaceSouth, sx, sy, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY + v, Z: box.MaxZ - 1}
	}, posMap, vis)
	box.Faces[skin.FaceWest] = buildFace(skin.FaceWest, sz, sy, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MinX, Y: box.MinY + v, Z: box.MinZ + u}
	}, posMap, vis)
	box.Faces[skin.FaceEast] = buildFace(skin.FaceEast, sz, sy, func(u, v int) skin.Vec3i {
		return skin.Vec3i{X: box.MaxX - 1, Y: box.MinY + v, Z: box.MinZ + u}
	}, posMap, vis)
}

func buildFace(faceDir int, w, h int, posAt func(u, v int) skin.Vec3i, posMap map[skin.Vec3i]*mergeEntry, vis []FaceVisibility) MergedFace {
	mf := MergedFace{Width: w, Height: h, Visible: true}
	mf.Colors = make([][]skin.PaintColor, h)
	allHidden := true
	for v := 0; v < h; v++ {
		mf.Colors[v] = make([]skin.PaintColor, w)
		for u := 0; u < w; u++ {
			ci, ok := posMap[posAt(u, v)]
			if !ok {
				mf.Visible = false
				return mf
			}
			mf.Colors[v][u] = ci.cube.FaceColors[faceDir]
			if vis[ci.idx].Visible[faceDir] {
				allHidden = false
			}
		}
	}
	if allHidden {
		mf.Visible = false
	}
	return mf
}
