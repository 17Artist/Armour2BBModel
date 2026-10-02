// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"fmt"
	"math"

	"github.com/17Artist/Armour2BBModel/skin"
)

// affine matrices use row-major storage, and column vectors.
type affine [16]float64

func identity() affine { return affine{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }
func multiply(a, b affine) affine {
	var c affine
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				c[i*4+j] += a[i*4+k] * b[k*4+j]
			}
		}
	}
	return c
}
func translation(x, y, z float64) affine { m := identity(); m[3], m[7], m[11] = x, y, z; return m }
func scaling(x, y, z float64) affine     { m := identity(); m[0], m[5], m[10] = x, y, z; return m }
func (m affine) point(x, y, z float64) [3]float64 {
	return [3]float64{m[0]*x + m[1]*y + m[2]*z + m[3], m[4]*x + m[5]*y + m[6]*z + m[7], m[8]*x + m[9]*y + m[10]*z + m[11]}
}
func partMatrix(p *skin.PartData) (affine, error) {
	if p.Matrix != nil {
		var m affine
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				m[i*4+j] = float64(p.Matrix[j*4+i])
				if math.IsNaN(m[i*4+j]) || math.IsInf(m[i*4+j], 0) {
					return affine{}, fmt.Errorf("non-finite transform matrix")
				}
			}
		}
		// AW's flattened matrices historically defaulted m33 to zero; positions use the affine 3x4.
		m[12], m[13], m[14], m[15] = 0, 0, 0, 1
		return m, nil
	}
	values := []float32{p.TranslateX, p.TranslateY, p.TranslateZ, p.RotationX, p.RotationY, p.RotationZ, p.ScaleX, p.ScaleY, p.ScaleZ, p.OffsetX, p.OffsetY, p.OffsetZ, p.PivotX, p.PivotY, p.PivotZ}
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return affine{}, fmt.Errorf("non-finite transform")
		}
	}
	// Zero-value PartData structs created by callers use identity scale, as legacy parts do.
	sx, sy, sz := float64(p.ScaleX), float64(p.ScaleY), float64(p.ScaleZ)
	if sx == 0 && sy == 0 && sz == 0 {
		sx, sy, sz = 1, 1, 1
	}
	if sx <= 0 || sy <= 0 || sz <= 0 {
		return affine{}, fmt.Errorf("zero or mirrored scale is unsupported")
	}
	rx, ry, rz := float64(p.RotationX)*math.Pi/180, float64(p.RotationY)*math.Pi/180, float64(p.RotationZ)*math.Pi/180
	cx, cy, cz, sinX, sinY, sinZ := math.Cos(rx), math.Cos(ry), math.Cos(rz), math.Sin(rx), math.Sin(ry), math.Sin(rz)
	rotation := affine{
		cz * cy, cz*sinY*sinX - sinZ*cx, cz*sinY*cx + sinZ*sinX, 0,
		sinZ * cy, sinZ*sinY*sinX + cz*cx, sinZ*sinY*cx - cz*sinX, 0,
		-sinY, cy * sinX, cy * cx, 0,
		0, 0, 0, 1,
	}
	m := translation(float64(p.TranslateX), float64(p.TranslateY), float64(p.TranslateZ))
	m = multiply(m, translation(float64(p.PivotX), float64(p.PivotY), float64(p.PivotZ)))
	m = multiply(m, rotation)
	m = multiply(m, translation(-float64(p.PivotX), -float64(p.PivotY), -float64(p.PivotZ)))
	m = multiply(m, scaling(sx, sy, sz))
	m = multiply(m, translation(float64(p.OffsetX), float64(p.OffsetY), float64(p.OffsetZ)))
	return m, nil
}

// mapTransformedBox keeps rotated/scaled cuboids as cuboids, rather than enlarging them to AABBs.
func mapTransformedBox(box MergedBox, m affine, origin [3]float32) (from, to, center, rotation [3]float32, err error) {
	// Change basis from AW to BB. Conjugation preserves face names/orientation in the local box.
	signs := [3]float64{-1, -1, 1}
	var basis [3][3]float64
	var scale [3]float64
	for col := 0; col < 3; col++ {
		for row := 0; row < 3; row++ {
			basis[row][col] = signs[row] * m[row*4+col] * signs[col]
			scale[col] += basis[row][col] * basis[row][col]
		}
		scale[col] = math.Sqrt(scale[col])
		if scale[col] < 1e-9 {
			return from, to, center, rotation, fmt.Errorf("degenerate part transform")
		}
		for row := 0; row < 3; row++ {
			basis[row][col] /= scale[col]
		}
	}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			dot := basis[0][i]*basis[0][j] + basis[1][i]*basis[1][j] + basis[2][i]*basis[2][j]
			if math.Abs(dot) > 1e-5 {
				return from, to, center, rotation, fmt.Errorf("part hierarchy creates shear; export without combined nonuniform scale and rotation")
			}
		}
	}
	det := basis[0][0]*(basis[1][1]*basis[2][2]-basis[1][2]*basis[2][1]) - basis[0][1]*(basis[1][0]*basis[2][2]-basis[1][2]*basis[2][0]) + basis[0][2]*(basis[1][0]*basis[2][1]-basis[1][1]*basis[2][0])
	if det < 0 {
		return from, to, center, rotation, fmt.Errorf("mirrored part matrix is unsupported")
	}
	c := m.point(float64(box.MinX+box.MaxX)/2, float64(box.MinY+box.MaxY)/2, float64(box.MinZ+box.MaxZ)/2)
	sizes := [3]float64{float64(box.MaxX - box.MinX), float64(box.MaxY - box.MinY), float64(box.MaxZ - box.MinZ)}
	for i := 0; i < 3; i++ {
		center[i] = float32(signs[i]*c[i]) + origin[i]
		half := float32(sizes[i] * scale[i] / 2)
		from[i], to[i] = center[i]-half, center[i]+half
	}
	y := math.Asin(math.Max(-1, math.Min(1, -basis[2][0])))
	var x, z float64
	if math.Abs(math.Cos(y)) > 1e-7 {
		x = math.Atan2(basis[2][1], basis[2][2])
		z = math.Atan2(basis[1][0], basis[0][0])
	} else {
		x = math.Atan2(-basis[1][2], basis[1][1])
	}
	rotation = [3]float32{float32(x * 180 / math.Pi), float32(y * 180 / math.Pi), float32(z * 180 / math.Pi)}
	for i := range rotation {
		if math.Abs(float64(rotation[i])) < 1e-5 {
			rotation[i] = 0
		}
	}
	return
}
