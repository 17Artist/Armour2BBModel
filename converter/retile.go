// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import "sort"

type retileRegion struct {
	lo, hi [3]int
}

// int is 32-bit in Go/WASM. Check products and sums in 64 bits, including
// extreme sparse extents; overflow must reject a cover rather than fill a hole.
func checkedRegionVolume(lo, hi [3]int) (uint64, bool) {
	const limit = uint64(1<<63 - 1)
	volume := uint64(1)
	for axis := 0; axis < 3; axis++ {
		if hi[axis] <= lo[axis] {
			return 0, false
		}
		width := uint64(hi[axis]) - uint64(lo[axis])
		if width > limit/volume {
			return 0, false
		}
		volume *= width
	}
	return volume, true
}

// A maximal greedy box can block a better boundary in its neighbors. Retiling
// three adjacent boxes into two uses exact volume tests, independent of the
// number of voxels in those boxes. It never fills holes or crosses materials.
func retileNeighbors(boxes []MergedBox) []MergedBox {
	canonicalBoxes(boxes)
	attempts := 0
	for pass := 0; pass < 3; pass++ {
		adjacent := make([][]int, len(boxes))
		for i := range boxes {
			for j := i + 1; j < len(boxes); j++ {
				if touchingBoxes(boxes[i], boxes[j]) {
					adjacent[i] = append(adjacent[i], j)
					adjacent[j] = append(adjacent[j], i)
				}
			}
		}
		used := make([]bool, len(boxes))
		var replacements []MergedBox
		for center, neighbors := range adjacent {
			if used[center] {
				continue
			}
			// Bound pathological junctions and overall work, with a fixed order.
			if len(neighbors) > 12 {
				neighbors = neighbors[:12]
			}
			found := false
			for a := 0; a < len(neighbors) && !found; a++ {
				i := neighbors[a]
				if used[i] {
					continue
				}
				for b := a + 1; b < len(neighbors); b++ {
					j := neighbors[b]
					if used[j] {
						continue
					}
					attempts++
					if attempts > 20000 {
						break
					}
					if result, ok := twoBoxCover([]MergedBox{boxes[center], boxes[i], boxes[j]}); ok {
						used[center], used[i], used[j] = true, true, true
						replacements = append(replacements, result...)
						found = true
						break
					}
				}
			}
			if attempts > 20000 {
				break
			}
		}
		if len(replacements) == 0 {
			break
		}
		for i, b := range boxes {
			if !used[i] {
				replacements = append(replacements, b)
			}
		}
		boxes = coalesceBoxes(replacements)
		canonicalBoxes(boxes)
		if attempts > 20000 {
			break
		}
	}
	return boxes
}

func touchingBoxes(a, b MergedBox) bool {
	if a.CubeType != b.CubeType {
		return false
	}
	la, ha := boxBounds(a)
	lb, hb := boxBounds(b)
	for axis := 0; axis < 3; axis++ {
		if ha[axis] != lb[axis] && hb[axis] != la[axis] {
			continue
		}
		p, q := (axis+1)%3, (axis+2)%3
		if la[p] < hb[p] && lb[p] < ha[p] && la[q] < hb[q] && lb[q] < ha[q] {
			return true
		}
	}
	return false
}

// Any two disjoint axis-aligned cuboids admit a separating coordinate plane.
// Candidate planes are source boundaries. A half is filled exactly when the
// sum of disjoint clipped volumes equals the volume of its bounding box.
func twoBoxCover(boxes []MergedBox) ([]MergedBox, bool) {
	if len(boxes) == 0 {
		return nil, false
	}
	var buffer [4]retileRegion
	regions := buffer[:0]
	if len(boxes) > len(buffer) {
		regions = make([]retileRegion, 0, len(boxes))
	}
	for _, box := range boxes {
		if box.CubeType != boxes[0].CubeType {
			return nil, false
		}
		lo, hi := boxBounds(box)
		regions = append(regions, retileRegion{lo, hi})
	}
	if result, ok := twoRegionCover(regions); ok {
		return []MergedBox{boundsBox(result[0].lo, result[0].hi, boxes[0].CubeType), boundsBox(result[1].lo, result[1].hi, boxes[0].CubeType)}, true
	}
	return nil, false
}

func twoRegionCover(regions []retileRegion) ([2]retileRegion, bool) {
	for axis := 0; axis < 3; axis++ {
		var buffer [8]int
		planes := buffer[:0]
		if len(regions)*2 > len(buffer) {
			planes = make([]int, 0, len(regions)*2)
		}
		for _, r := range regions {
			planes = append(planes, r.lo[axis], r.hi[axis])
		}
		sort.Ints(planes)
		for pi, plane := range planes {
			if pi > 0 && plane == planes[pi-1] {
				continue
			}
			left, ok := filledRegionHalf(regions, axis, plane, true)
			if !ok {
				continue
			}
			right, ok := filledRegionHalf(regions, axis, plane, false)
			if ok {
				return [2]retileRegion{left, right}, true
			}
		}
	}
	return [2]retileRegion{}, false
}

func filledHalf(boxes []MergedBox, axis, plane int, left bool) (MergedBox, bool) {
	if len(boxes) == 0 {
		return MergedBox{}, false
	}
	regions := make([]retileRegion, len(boxes))
	for i, b := range boxes {
		regions[i].lo, regions[i].hi = boxBounds(b)
	}
	r, ok := filledRegionHalf(regions, axis, plane, left)
	return boundsBox(r.lo, r.hi, boxes[0].CubeType), ok
}

// Input regions are disjoint filled boxes. Their clipped volume equals their
// bounding volume exactly iff that half has no hole. No voxel grid is needed.
func filledRegionHalf(regions []retileRegion, axis, plane int, left bool) (retileRegion, bool) {
	var result retileRegion
	volume := uint64(0)
	for _, b := range regions {
		lo, hi := b.lo, b.hi
		if left {
			if hi[axis] > plane {
				hi[axis] = plane
			}
		} else {
			if lo[axis] < plane {
				lo[axis] = plane
			}
		}
		if lo[axis] >= hi[axis] {
			continue
		}
		if volume == 0 {
			result.lo, result.hi = lo, hi
		} else {
			for a := 0; a < 3; a++ {
				if lo[a] < result.lo[a] {
					result.lo[a] = lo[a]
				}
				if hi[a] > result.hi[a] {
					result.hi[a] = hi[a]
				}
			}
		}
		v, ok := checkedRegionVolume(lo, hi)
		if !ok || v > uint64(1<<63-1)-volume {
			return retileRegion{}, false
		}
		volume += v
	}
	v, ok := checkedRegionVolume(result.lo, result.hi)
	return result, ok && volume > 0 && volume == v
}

// A three-box guillotine cover first separates a filled half, then separates
// the remainder into two filled halves. This is a safe improvement candidate,
// not an exhaustive search: some three-box covers have no separating plane.
func threeRegionCover(regions []retileRegion) ([3]retileRegion, bool) {
	// Callers provide at most four mutually disjoint, positive-extent regions.
	if len(regions) == 0 || len(regions) > 4 {
		return [3]retileRegion{}, false
	}
	for axis := 0; axis < 3; axis++ {
		var planes [8]int
		for i, r := range regions {
			planes[2*i], planes[2*i+1] = r.lo[axis], r.hi[axis]
		}
		sort.Ints(planes[:2*len(regions)])
		for pi, plane := range planes[:2*len(regions)] {
			if pi > 0 && plane == planes[pi-1] {
				continue
			}
			for side := 0; side < 2; side++ {
				left := side == 0
				one, ok := filledRegionHalf(regions, axis, plane, left)
				if !ok {
					continue
				}
				var clipped [4]retileRegion
				n := 0
				for _, r := range regions {
					if left {
						if r.lo[axis] < plane {
							r.lo[axis] = plane
						}
					} else if r.hi[axis] > plane {
						r.hi[axis] = plane
					}
					if r.lo[axis] < r.hi[axis] {
						clipped[n] = r
						n++
					}
				}
				if n == 0 {
					continue
				}
				if two, ok := twoRegionCover(clipped[:n]); ok {
					return [3]retileRegion{one, two[0], two[1]}, true
				}
			}
		}
	}
	return [3]retileRegion{}, false
}

// Keep search work fixed independently of coordinate span and source volume.
// Stars cheaply find most improvements; a smaller connected search also visits
// path-shaped neighborhoods that do not have one box touching the other three.
func retileFourNeighbors(boxes []MergedBox) []MergedBox {
	boxes = retileFourSearch(boxes, 2500, false)
	return retileFourSearch(boxes, 500, true)
}

func retileFourSearch(boxes []MergedBox, budget int, connected bool) []MergedBox {
	canonicalBoxes(boxes)
	attempts := 0
	for pass := 0; pass < 3; pass++ {
		adjacent := make([][]int, len(boxes))
		for i := range boxes {
			for j := i + 1; j < len(boxes); j++ {
				if touchingBoxes(boxes[i], boxes[j]) {
					adjacent[i] = append(adjacent[i], j)
					adjacent[j] = append(adjacent[j], i)
				}
			}
		}
		if connected {
			for i := range adjacent {
				if len(adjacent[i]) > 12 {
					adjacent[i] = adjacent[i][:12]
				}
			}
		}
		used := make([]bool, len(boxes))
		seen := make(map[[4]int]bool)
		var replacements []MergedBox
		try := func(ids [4]int) bool {
			sort.Ints(ids[:])
			if seen[ids] {
				return false
			}
			seen[ids] = true
			attempts++
			if attempts > budget {
				return false
			}
			var regions [4]retileRegion
			for n, i := range ids {
				regions[n].lo, regions[n].hi = boxBounds(boxes[i])
			}
			result, ok := threeRegionCover(regions[:])
			if !ok {
				return false
			}
			for _, i := range ids {
				used[i] = true
			}
			for _, r := range result {
				replacements = append(replacements, boundsBox(r.lo, r.hi, boxes[ids[0]].CubeType))
			}
			return true
		}
		for center, neighbors := range adjacent {
			if used[center] {
				continue
			}
			found := false
			if !connected {
				if len(neighbors) > 10 {
					neighbors = neighbors[:10]
				}
				for a := 0; a < len(neighbors) && !found; a++ {
					i := neighbors[a]
					if used[i] {
						continue
					}
					for b := a + 1; b < len(neighbors) && !found; b++ {
						j := neighbors[b]
						if used[j] {
							continue
						}
						for c := b + 1; c < len(neighbors); c++ {
							k := neighbors[c]
							if !used[k] && try([4]int{center, i, j, k}) {
								found = true
								break
							}
							if attempts > budget {
								break
							}
						}
						if attempts > budget {
							break
						}
					}
					if attempts > budget {
						break
					}
				}
			} else {
				for _, i := range neighbors {
					if i < center || used[i] {
						continue
					}
					frontier := append(append([]int(nil), neighbors...), adjacent[i]...)
					sort.Ints(frontier)
					for jj, j := range frontier {
						if j < center || j == i || j == center || used[j] || (jj > 0 && j == frontier[jj-1]) {
							continue
						}
						frontier2 := append(append([]int(nil), frontier...), adjacent[j]...)
						sort.Ints(frontier2)
						for kk, k := range frontier2 {
							if k < center || k == i || k == j || k == center || used[k] || (kk > 0 && k == frontier2[kk-1]) {
								continue
							}
							if try([4]int{center, i, j, k}) {
								found = true
								break
							}
							if attempts > budget {
								break
							}
						}
						if found || attempts > budget {
							break
						}
					}
					if found || attempts > budget {
						break
					}
				}
			}
			if attempts > budget {
				break
			}
		}
		if len(replacements) == 0 {
			break
		}
		for i, b := range boxes {
			if !used[i] {
				replacements = append(replacements, b)
			}
		}
		boxes = retileNeighbors(coalesceBoxes(replacements))
		canonicalBoxes(boxes)
		if attempts > budget {
			break
		}
	}
	return boxes
}
