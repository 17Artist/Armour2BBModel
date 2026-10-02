// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"

	"github.com/17Artist/Armour2BBModel/skin"
)

// TextureAtlas 纹理图集
type TextureAtlas struct {
	regions []atlasRegion
	width   int
	height  int
}

type atlasRegion struct {
	x, y   int
	w, h   int
	pixels [][]skin.PaintColor // [h][w] 已按 Blockbench UV 方向排列
	mask   [][]bool
	alpha  byte
}

func NewTextureAtlas() *TextureAtlas {
	return &TextureAtlas{}
}

// AddFace 添加一个面到图集。
// awFace: AW 面方向索引。
// 颜色数据会被重新排列为 Blockbench 期望的 UV 方向。
//
// Blockbench per-face UV 方向（官方 CubeFace.UVToLocal）：
//
//	north(Z-): U=-X, V=-Y(上到下)
//	south(Z+): U=+X, V=-Y(上到下)
//	east(X+):  U=-Z(前到后), V=-Y(上到下)
//	west(X-):  U=+Z(后到前), V=-Y(上到下)
//	up(Y+):    U=+X(左到右), V=+Z(前到后)
//	down(Y-):  U=+X(左到右), V=-Z(后到前)
//
// buildFace 中颜色排列（AW 空间，翻转前）：
//
//	down/up:     u=+X, v=+Z
//	north/south: u=+X, v=+Y
//	west/east:   u=+Z, v=+Y
//
// AW->BB 坐标翻转：X反转, Y反转, Z不变
// 所以 AW 空间的 +X 在 BB 空间是 -X，+Y 在 BB 空间是 -Y
func (a *TextureAtlas) AddFace(face MergedFace, awFace int) int {
	if !face.Visible || face.Width == 0 || face.Height == 0 {
		return -1
	}

	w, h := face.Width, face.Height
	src := face.Colors

	// 根据 AW 面方向和坐标翻转，重新排列像素为 BB UV 方向
	// AW face -> BB face 映射: down->up, up->down, north->north, south->south, west->east, east->west
	var pixels [][]skin.PaintColor

	switch awFace {
	case skin.FaceDown:
		// AW down -> BB up
		// AW: u=+X, v=+Z; BB up: U=+X, V=+Z
		// X 翻转: U 方向反转; Z 不变: V 不变
		pixels = flipH(src, w, h)

	case skin.FaceUp:
		// AW up -> BB down
		// AW: u=+X, v=+Z; BB down: U=+X, V=-Z
		// X 翻转: U 反转; Z 不变但 V 方向反了: V 反转
		pixels = flipHV(src, w, h)

	case skin.FaceNorth:
		// AW north -> BB north
		// AW +X -> BB -X matches north U; AW +Y -> BB -Y matches V.
		pixels = copyPixels(src, w, h)

	case skin.FaceSouth:
		// AW south -> BB south
		// South U is BB +X, opposite to AW +X; V matches.
		pixels = flipH(src, w, h)

	case skin.FaceWest:
		// AW west -> BB east
		// AW: u=+Z, v=+Y; BB east: U=-Z, V=-Y
		// Z 不变 + U=-Z: U 反转; Y 翻转 + V=-Y: 抵消，V 不变
		pixels = flipH(src, w, h)

	case skin.FaceEast:
		// AW east -> BB west
		// AW: u=+Z, v=+Y; BB west: U=+Z, V=-Y
		// Z 不变: U 不变; Y 翻转 + V=-Y: 抵消，V 不变
		pixels = copyPixels(src, w, h)
	}

	idx := len(a.regions)
	var mask [][]bool
	if face.Mask != nil {
		mask = make([][]bool, h)
		for v := 0; v < h; v++ {
			mask[v] = make([]bool, w)
			for u := 0; u < w; u++ {
				x, y := u, v
				if awFace == skin.FaceDown || awFace == skin.FaceUp || awFace == skin.FaceSouth || awFace == skin.FaceWest {
					x = w - 1 - u
				}
				if awFace == skin.FaceUp {
					y = h - 1 - v
				}
				mask[v][u] = face.Mask[y][x]
			}
		}
	}
	alpha := face.Alpha
	if alpha == 0 {
		alpha = 255
	}
	a.regions = append(a.regions, atlasRegion{w: w, h: h, pixels: pixels, mask: mask, alpha: alpha})
	return idx
}

func copyPixels(src [][]skin.PaintColor, w, h int) [][]skin.PaintColor {
	dst := make([][]skin.PaintColor, h)
	for v := 0; v < h; v++ {
		dst[v] = make([]skin.PaintColor, w)
		copy(dst[v], src[v])
	}
	return dst
}

func flipH(src [][]skin.PaintColor, w, h int) [][]skin.PaintColor {
	dst := make([][]skin.PaintColor, h)
	for v := 0; v < h; v++ {
		dst[v] = make([]skin.PaintColor, w)
		for u := 0; u < w; u++ {
			dst[v][u] = src[v][w-1-u]
		}
	}
	return dst
}

func flipHV(src [][]skin.PaintColor, w, h int) [][]skin.PaintColor {
	dst := make([][]skin.PaintColor, h)
	for v := 0; v < h; v++ {
		dst[v] = make([]skin.PaintColor, w)
		for u := 0; u < w; u++ {
			dst[v][u] = src[h-1-v][w-1-u]
		}
	}
	return dst
}

// Pack 使用 shelf packing 算法排列纹理区域
func (a *TextureAtlas) Pack() {
	if len(a.regions) == 0 {
		a.width = 1
		a.height = 1
		return
	}

	x, y, rowH := 0, 0, 0
	maxW := 0
	totalArea := 0
	for i := range a.regions {
		totalArea += a.regions[i].w * a.regions[i].h
	}
	targetW := 1
	for targetW*targetW < totalArea*2 {
		targetW++
	}
	if targetW < 16 {
		targetW = 16
	}

	for i := range a.regions {
		r := &a.regions[i]
		if x+r.w > targetW {
			y += rowH
			x = 0
			rowH = 0
		}
		r.x = x
		r.y = y
		x += r.w
		if r.h > rowH {
			rowH = r.h
		}
		if x > maxW {
			maxW = x
		}
	}
	a.width = maxW
	a.height = y + rowH
	if a.width == 0 {
		a.width = 1
	}
	if a.height == 0 {
		a.height = 1
	}
}

// GetUV 始终返回正常方向的 UV（u1 < u2, v1 < v2）
func (a *TextureAtlas) GetUV(regionIndex int) [4]float32 {
	if regionIndex < 0 || regionIndex >= len(a.regions) {
		return [4]float32{}
	}
	r := a.regions[regionIndex]
	return [4]float32{
		float32(r.x), float32(r.y),
		float32(r.x + r.w), float32(r.y + r.h),
	}
}

func (a *TextureAtlas) RegionCount() int { return len(a.regions) }
func (a *TextureAtlas) Width() int       { return a.width }
func (a *TextureAtlas) Height() int      { return a.height }

func (a *TextureAtlas) GenerateBase64PNG() (string, error) {
	img := image.NewNRGBA(image.Rect(0, 0, a.width, a.height))
	for _, r := range a.regions {
		for v := 0; v < r.h && v < len(r.pixels); v++ {
			for u := 0; u < r.w && u < len(r.pixels[v]); u++ {
				if r.mask != nil && !r.mask[v][u] {
					continue
				}
				c := r.pixels[v][u]
				img.SetNRGBA(r.x+u, r.y+v, color.NRGBA{R: c.R, G: c.G, B: c.B, A: r.alpha})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
