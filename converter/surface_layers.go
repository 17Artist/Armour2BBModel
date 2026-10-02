package converter

import (
	"math"
	"sort"

	"github.com/17Artist/Armour2BBModel/skin"
)

type staticSurfaceWork struct {
	boxes  []MergedBox
	matrix affine
	origin [3]float32
	order  int
}

type staticFaceKey struct {
	center [3]int64
	normal [3]int8
}
type staticFaceOwner struct {
	face *MergedFace
	u, v int
}

// Integer, unit-scale rigid rotations permit exact surface matching. Arbitrary
// transforms and glass are left intact. This masks duplicate surface pixels;
// no cuboid, voxel, coordinate, source group, or unique surface is removed.
// Original source part order determines which fixed attachment paints a shared
// surface, so differently colored layers have deterministic appearance too.
func normalizeStaticSurfaces(works []staticSurfaceWork) int {
	sort.SliceStable(works, func(i, j int) bool { return works[i].order < works[j].order })
	owners := map[staticFaceKey]staticFaceOwner{}
	removed, visited := 0, 0
	for _, work := range works {
		if !unitGridTransform(work.matrix, work.origin) {
			continue
		}
		for bi := range work.boxes {
			box := &work.boxes[bi]
			if box.CubeType.IsGlass() {
				continue
			}
			for dir := range box.Faces {
				face := &box.Faces[dir]
				if !face.Visible {
					continue
				}
				offset := skin.FaceOffset(dir)
				normal := [3]int8{}
				for row, sign := range [3]float64{-1, -1, 1} {
					normal[row] = int8(math.Round(sign * (work.matrix[row*4]*float64(offset.X) + work.matrix[row*4+1]*float64(offset.Y) + work.matrix[row*4+2]*float64(offset.Z))))
				}
				for v := 0; v < face.Height; v++ {
					for u := 0; u < face.Width; u++ {
						if face.Mask != nil && !face.Mask[v][u] {
							continue
						}
						visited++
						if visited > 2_000_000 {
							return removed
						}
						p := surfaceVoxel(*box, dir, u, v)
						world := work.matrix.point(float64(p.X)+.5+float64(offset.X)*.5, float64(p.Y)+.5+float64(offset.Y)*.5, float64(p.Z)+.5+float64(offset.Z)*.5)
						key := staticFaceKey{normal: normal}
						for axis, sign := range [3]float64{-1, -1, 1} {
							key.center[axis] = int64(math.Round(2 * (sign*world[axis] + float64(work.origin[axis]))))
						}
						if previous, ok := owners[key]; ok {
							ensureFaceMask(previous.face)
							previous.face.Mask[previous.v][previous.u] = false
							removed++
						}
						owners[key] = staticFaceOwner{face, u, v}
					}
				}
			}
		}
	}
	for _, work := range works {
		for bi := range work.boxes {
			for dir := range work.boxes[bi].Faces {
				face := &work.boxes[bi].Faces[dir]
				if face.Mask == nil {
					continue
				}
				any := false
				for _, row := range face.Mask {
					for _, visible := range row {
						any = any || visible
					}
				}
				face.Visible = any
			}
		}
	}
	return removed
}

func ensureFaceMask(face *MergedFace) {
	if face.Mask != nil {
		return
	}
	face.Mask = make([][]bool, face.Height)
	for v := range face.Mask {
		face.Mask[v] = make([]bool, face.Width)
		for u := range face.Mask[v] {
			face.Mask[v][u] = true
		}
	}
}

func unitGridTransform(m affine, origin [3]float32) bool {
	for row := 0; row < 3; row++ {
		nonzero := 0
		for col := 0; col < 3; col++ {
			v := m[row*4+col]
			if math.Abs(v) < 1e-6 {
				continue
			}
			if math.Abs(math.Abs(v)-1) > 1e-6 {
				return false
			}
			nonzero++
		}
		if nonzero != 1 {
			return false
		}
		v := m[row*4+3] + float64(origin[row])
		if math.Abs(v-math.Round(v)) > 1e-5 || math.Abs(v) > 1e12 {
			return false
		}
	}
	return true
}

func surfaceVoxel(box MergedBox, dir, u, v int) skin.Vec3i {
	switch dir {
	case skin.FaceDown:
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY, Z: box.MinZ + v}
	case skin.FaceUp:
		return skin.Vec3i{X: box.MinX + u, Y: box.MaxY - 1, Z: box.MinZ + v}
	case skin.FaceNorth:
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY + v, Z: box.MinZ}
	case skin.FaceSouth:
		return skin.Vec3i{X: box.MinX + u, Y: box.MinY + v, Z: box.MaxZ - 1}
	case skin.FaceWest:
		return skin.Vec3i{X: box.MinX, Y: box.MinY + v, Z: box.MinZ + u}
	default:
		return skin.Vec3i{X: box.MaxX - 1, Y: box.MinY + v, Z: box.MinZ + u}
	}
}
