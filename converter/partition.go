// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"math/bits"
	"sort"

	"github.com/17Artist/Armour2BBModel/skin"
)

var axisOrders = [6][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

func coord(p skin.Vec3i, axis int) int {
	switch axis {
	case 0:
		return p.X
	case 1:
		return p.Y
	default:
		return p.Z
	}
}

func boxBounds(b MergedBox) (lo, hi [3]int) {
	return [3]int{b.MinX, b.MinY, b.MinZ}, [3]int{b.MaxX, b.MaxY, b.MaxZ}
}

func boundsBox(lo, hi [3]int, ct skin.CubeType) MergedBox {
	return MergedBox{MinX: lo[0], MinY: lo[1], MinZ: lo[2], MaxX: hi[0], MaxY: hi[1], MaxZ: hi[2], CubeType: ct}
}

func boxVolume(b MergedBox) int { return b.SizeX() * b.SizeY() * b.SizeZ() }

func canonicalBoxes(boxes []MergedBox) {
	sort.Slice(boxes, func(i, j int) bool {
		a, b := boxes[i], boxes[j]
		x := [7]int{int(a.CubeType), a.MinX, a.MinY, a.MinZ, a.MaxX, a.MaxY, a.MaxZ}
		y := [7]int{int(b.CubeType), b.MinX, b.MinY, b.MinZ, b.MaxX, b.MaxY, b.MaxZ}
		for k := range x {
			if x[k] != y[k] {
				return x[k] < y[k]
			}
		}
		return false
	})
}

func partitionComponents(cubes []skin.CubeData) []MergedBox {
	positions := make(map[skin.Vec3i]int, len(cubes))
	for i, c := range cubes {
		positions[c.Pos] = i
	}
	visited := make([]bool, len(cubes))
	var result []MergedBox
	for i, c := range cubes {
		if visited[i] {
			continue
		}
		visited[i] = true
		component := []skin.CubeData{c}
		for head := 0; head < len(component); head++ {
			p := component[head].Pos
			for f := 0; f < 6; f++ {
				d := skin.FaceOffset(f)
				n := skin.Vec3i{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
				j, ok := positions[n]
				if ok && !visited[j] && cubes[j].Type == c.Type {
					visited[j] = true
					component = append(component, cubes[j])
				}
			}
		}
		result = append(result, partitionComponent(component)...)
	}
	canonicalBoxes(result)
	return result
}

func partitionComponent(cubes []skin.CubeData) []MergedBox {
	// A fixed voxel order also makes budgeted exact search independent of the
	// source file's cube ordering and its breadth-first traversal seed.
	cubes = append([]skin.CubeData(nil), cubes...)
	sort.Slice(cubes, func(i, j int) bool {
		a, b := cubes[i].Pos, cubes[j].Pos
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.Z < b.Z
	})
	positions := make(map[skin.Vec3i]int, len(cubes))
	lo, hi := [3]int{cubes[0].Pos.X, cubes[0].Pos.Y, cubes[0].Pos.Z}, [3]int{cubes[0].Pos.X + 1, cubes[0].Pos.Y + 1, cubes[0].Pos.Z + 1}
	for i, c := range cubes {
		positions[c.Pos] = i
		for a := 0; a < 3; a++ {
			v := coord(c.Pos, a)
			if v < lo[a] {
				lo[a] = v
			}
			if v+1 > hi[a] {
				hi[a] = v + 1
			}
		}
	}
	whole := boundsBox(lo, hi, cubes[0].Type)
	if volume, ok := checkedRegionVolume(lo, hi); ok && volume == uint64(len(cubes)) {
		return []MergedBox{whole}
	}
	var best []MergedBox
	for _, order := range axisOrders {
		for direction := 0; direction < 8; direction++ {
			candidate := coalesceBoxes(directionalPartition(cubes, positions, order, direction))
			if best == nil || len(candidate) < len(best) {
				best = candidate
			}
		}
	}
	if len(cubes) <= 32 && len(best) > 2 {
		best = exactSmallPartition(cubes, positions, best)
	}
	if len(best) >= 3 && len(best) <= 512 {
		best = retileNeighbors(best)
	}
	if len(cubes) > 32 && len(best) >= 4 && len(best) <= 512 {
		best = retileFourNeighbors(best)
	}
	canonicalBoxes(best)
	return best
}

// Sparse scans avoid allocating a volume proportional to the coordinate span.
// Sorting is the reverse of growth order, including signed axis directions.
func directionalPartition(cubes []skin.CubeData, positions map[skin.Vec3i]int, order [3]int, direction int) []MergedBox {
	indices := make([]int, len(cubes))
	for i := range indices {
		indices[i] = i
	}
	signs := [3]int{1, 1, 1}
	for a := 0; a < 3; a++ {
		if direction&(1<<a) != 0 {
			signs[a] = -1
		}
	}
	sort.Slice(indices, func(i, j int) bool {
		for k := 2; k >= 0; k-- {
			a := order[k]
			x, y := coord(cubes[indices[i]].Pos, a), coord(cubes[indices[j]].Pos, a)
			if x != y {
				return x*signs[a] < y*signs[a]
			}
		}
		return false
	})
	used := make([]bool, len(cubes))
	var boxes []MergedBox
	for _, idx := range indices {
		if used[idx] {
			continue
		}
		c := cubes[idx]
		lo, hi := [3]int{c.Pos.X, c.Pos.Y, c.Pos.Z}, [3]int{c.Pos.X + 1, c.Pos.Y + 1, c.Pos.Z + 1}
		for _, axis := range order {
			for {
				slabLo, slabHi := lo, hi
				if signs[axis] > 0 {
					slabLo[axis] = hi[axis]
					slabHi[axis] = hi[axis] + 1
				} else {
					slabLo[axis] = lo[axis] - 1
					slabHi[axis] = lo[axis]
				}
				if !slabAvailable(slabLo, slabHi, positions, used) {
					break
				}
				if signs[axis] > 0 {
					hi[axis]++
				} else {
					lo[axis]--
				}
			}
		}
		for x := lo[0]; x < hi[0]; x++ {
			for y := lo[1]; y < hi[1]; y++ {
				for z := lo[2]; z < hi[2]; z++ {
					used[positions[skin.Vec3i{X: x, Y: y, Z: z}]] = true
				}
			}
		}
		boxes = append(boxes, boundsBox(lo, hi, c.Type))
	}
	return boxes
}

func slabAvailable(lo, hi [3]int, positions map[skin.Vec3i]int, used []bool) bool {
	for x := lo[0]; x < hi[0]; x++ {
		for y := lo[1]; y < hi[1]; y++ {
			for z := lo[2]; z < hi[2]; z++ {
				i, ok := positions[skin.Vec3i{X: x, Y: y, Z: z}]
				if !ok || used[i] {
					return false
				}
			}
		}
	}
	return true
}

// Join only complete, equal cross-sections. Every union stays filled and disjoint.
func coalesceBoxes(boxes []MergedBox) []MergedBox {
	for {
		before := len(boxes)
		for axis := 0; axis < 3; axis++ {
			a, b := (axis+1)%3, (axis+2)%3
			sort.Slice(boxes, func(i, j int) bool {
				li, hi := boxBounds(boxes[i])
				lj, hj := boxBounds(boxes[j])
				x := [6]int{int(boxes[i].CubeType), li[a], hi[a], li[b], hi[b], li[axis]}
				y := [6]int{int(boxes[j].CubeType), lj[a], hj[a], lj[b], hj[b], lj[axis]}
				for k := range x {
					if x[k] != y[k] {
						return x[k] < y[k]
					}
				}
				return false
			})
			out := boxes[:0]
			for _, box := range boxes {
				if len(out) > 0 {
					last := &out[len(out)-1]
					l, h := boxBounds(*last)
					n, m := boxBounds(box)
					if last.CubeType == box.CubeType && l[a] == n[a] && h[a] == m[a] && l[b] == n[b] && h[b] == m[b] && h[axis] == n[axis] {
						h[axis] = m[axis]
						*last = boundsBox(l, h, last.CubeType)
						continue
					}
				}
				out = append(out, box)
			}
			boxes = out
		}
		if len(boxes) == before {
			return boxes
		}
	}
}

// All filled integer cuboids are candidates for <=32 voxels. A deterministic
// node budget bounds browser latency. Exhaustion retains the best valid cover,
// so a timed-out search never makes the heuristic solution worse.
func exactSmallPartition(cubes []skin.CubeData, positions map[skin.Vec3i]int, incumbent []MergedBox) []MergedBox {
	type candidate struct {
		box  MergedBox
		mask uint64
	}
	var candidates []candidate
	for _, c := range cubes {
		for _, d := range cubes {
			lo := [3]int{c.Pos.X, c.Pos.Y, c.Pos.Z}
			hi := [3]int{d.Pos.X + 1, d.Pos.Y + 1, d.Pos.Z + 1}
			if hi[0] <= lo[0] || hi[1] <= lo[1] || hi[2] <= lo[2] {
				continue
			}
			box := boundsBox(lo, hi, c.Type)
			if volume, ok := checkedRegionVolume(lo, hi); !ok || volume > uint64(len(cubes)) {
				continue
			}
			var mask uint64
			valid := true
			for x := lo[0]; x < hi[0] && valid; x++ {
				for y := lo[1]; y < hi[1] && valid; y++ {
					for z := lo[2]; z < hi[2]; z++ {
						i, ok := positions[skin.Vec3i{X: x, Y: y, Z: z}]
						if !ok {
							valid = false
							break
						}
						mask |= 1 << i
					}
				}
			}
			if valid {
				candidates = append(candidates, candidate{box, mask})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return bits.OnesCount64(candidates[i].mask) > bits.OnesCount64(candidates[j].mask)
	})
	byVoxel := make([][]int, len(cubes))
	for i, c := range candidates {
		for m := c.mask; m != 0; m &= m - 1 {
			v := bits.TrailingZeros64(m)
			byVoxel[v] = append(byVoxel[v], i)
		}
	}
	best := append([]MergedBox(nil), incumbent...)
	var path []MergedBox
	memo := make(map[uint64]int)
	nodes := 0
	var search func(uint64)
	search = func(remaining uint64) {
		nodes++
		if nodes > 25000 {
			return
		}
		if remaining == 0 {
			if len(path) < len(best) {
				best = append([]MergedBox(nil), path...)
			}
			return
		}
		if len(path)+1 >= len(best) {
			return
		}
		if depth, ok := memo[remaining]; ok && depth <= len(path) {
			return
		}
		memo[remaining] = len(path)
		chosen, minChoices, maxVolume := -1, len(candidates)+1, 1
		for m := remaining; m != 0; m &= m - 1 {
			v := bits.TrailingZeros64(m)
			choices := 0
			for _, i := range byVoxel[v] {
				c := candidates[i]
				if c.mask&remaining == c.mask {
					choices++
					if vol := bits.OnesCount64(c.mask); vol > maxVolume {
						maxVolume = vol
					}
				}
			}
			if choices < minChoices {
				chosen = v
				minChoices = choices
			}
		}
		if len(path)+(bits.OnesCount64(remaining)+maxVolume-1)/maxVolume >= len(best) {
			return
		}
		for _, i := range byVoxel[chosen] {
			c := candidates[i]
			if c.mask&remaining != c.mask {
				continue
			}
			path = append(path, c.box)
			search(remaining &^ c.mask)
			path = path[:len(path)-1]
			if nodes > 25000 {
				return
			}
		}
	}
	search((uint64(1) << len(cubes)) - 1)
	return best
}
