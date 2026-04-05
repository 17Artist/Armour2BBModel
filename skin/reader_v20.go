// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

func readV20(r io.Reader, fileVersion int) (*SkinFile, error) {
	skin := &SkinFile{FileVersion: fileVersion}

	readInt32(r)
	readInt32(r)

	ctx := &v20Context{}
	if err := ctx.readSkinChunk(r, skin); err != nil {
		return nil, err
	}

	return skin, nil
}

type v20Context struct {
	palette  []PaintColor
	geometry []v20GeomEntry
}

type v20GeomEntry struct {
	x, y, z    int8
	faceColors [6]PaintColor
}

func (ctx *v20Context) readSkinChunk(r io.Reader, skin *SkinFile) error {
	typeName, err := readVarString(r)
	if err != nil {
		return fmt.Errorf("reading skin type: %w", err)
	}
	skin.SkinType = typeName

	for {
		header, err := readChunkHeader(r)
		if err != nil || header == nil {
			break
		}

		data, err := readChunkBody(r, header)
		if err != nil {
			return err
		}
		if data == nil {
			continue
		}

		br := bytes.NewReader(data)
		switch header.name {
		case "PPTS":
			ctx.readPropsChunk(br, skin)
		case "PALE":
			ctx.readPaletteChunk(br)
		case "CCBO":
			ctx.readGeometryChunk(br)
		case "SKPR":
			ctx.readPartChunk(br, skin)
		case "PADT":
			ctx.readPaintChunk(br, skin)
		}
		// 其他 chunk（SET2/SET3/SET4/ANIM/FILE/VCBO 等）跳过
	}

	return nil
}

type chunkHeader struct {
	length int32
	name   string
	flag   int16
}

func readChunkHeader(r io.Reader) (*chunkHeader, error) {
	length, err := readInt32(r)
	if err != nil {
		return nil, nil // EOF
	}
	if length == 0 {
		return nil, nil
	}

	nameBytes := make([]byte, 4)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return nil, err
	}
	flag, err := readInt16(r)
	if err != nil {
		return nil, err
	}

	return &chunkHeader{length: length, name: string(nameBytes), flag: flag}, nil
}

func readChunkBody(r io.Reader, h *chunkHeader) ([]byte, error) {
	dataLen := int(h.length) - 4 - 2
	if dataLen <= 0 {
		return nil, nil
	}

	raw := make([]byte, dataLen)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}

	encrypted := h.flag&1 != 0
	gzipped := h.flag&2 != 0

	if encrypted {
		return nil, nil // 不支持加密
	}

	if gzipped {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}

	return raw, nil
}

func (ctx *v20Context) readPropsChunk(r io.Reader, skin *SkinFile) {
	count, err := readVarInt(r)
	if err != nil {
		return
	}
	props := make(map[string]string, count)
	for i := 0; i < count; i++ {
		key, _ := readVarString(r)
		value, _ := readVarString(r)
		props[key] = value
	}
	skin.Properties = props
}

func (ctx *v20Context) readPaletteChunk(r io.Reader) {
	readVarInt(r) // options
	readVarInt(r) // reserved

	groupCount, _ := readVarInt(r)
	for i := 0; i < groupCount; i++ {
		paintType, _ := readByte(r)
		usedBytes, _ := readByte(r)

		entryCount, _ := readVarInt(r)
		for j := 0; j < entryCount; j++ {
			if usedBytes == 3 {
				cr, _ := readByte(r)
				cg, _ := readByte(r)
				cb, _ := readByte(r)
				ctx.palette = append(ctx.palette, PaintColor{R: cr, G: cg, B: cb, PaintType: paintType})
			} else if usedBytes == 4 {
				v, _ := readInt32(r)
				ctx.palette = append(ctx.palette, PaintColor{
					R: byte(v >> 16), G: byte(v >> 8), B: byte(v), PaintType: byte(v >> 24),
				})
			} else {
				buf := make([]byte, usedBytes)
				io.ReadFull(r, buf)
				ctx.palette = append(ctx.palette, EmptyColor)
			}
		}
	}
}

func (ctx *v20Context) readGeometryChunk(r io.Reader) {
	readVarInt(r) // id
	readVarInt(r) // options

	count, _ := readVarInt(r)
	for i := 0; i < count; i++ {
		x, _ := readByte(r)
		y, _ := readByte(r)
		z, _ := readByte(r)

		entry := v20GeomEntry{x: int8(x), y: int8(y), z: int8(z)}
		for face := 0; face < 6; face++ {
			faceOpt, _ := readByte(r)
			if faceOpt&0x80 != 0 {
				colorIdx, _ := readVarInt(r)
				if colorIdx >= 0 && colorIdx < len(ctx.palette) {
					entry.faceColors[face] = ctx.palette[colorIdx]
				}
			}
		}
		ctx.geometry = append(ctx.geometry, entry)
	}
}

func (ctx *v20Context) readPartChunk(r io.Reader, skin *SkinFile) {
	readVarInt(r) // partId
	readVarInt(r) // parentId
	name, _ := readVarString(r)
	typeName, _ := readVarString(r)

	part := &PartData{
		PartType: fixPartTypeName(typeName),
		Name:     name,
		ScaleX:   1, ScaleY: 1, ScaleZ: 1,
	}

	// transform (9 floats)
	tx, _ := readFloat32(r)
	ty, _ := readFloat32(r)
	tz, _ := readFloat32(r)
	rx, _ := readFloat32(r)
	ry, _ := readFloat32(r)
	rz, _ := readFloat32(r)
	sx, _ := readFloat32(r)
	sy, _ := readFloat32(r)
	sz, _ := readFloat32(r)
	part.TranslateX, part.TranslateY, part.TranslateZ = tx, ty, tz
	part.RotationX, part.RotationY, part.RotationZ = rx, ry, rz
	part.ScaleX, part.ScaleY, part.ScaleZ = sx, sy, sz

	// geometry selectors
	selCount, _ := readVarInt(r)
	for i := 0; i < selCount; i++ {
		startIdx, _ := readInt32(r)
		endIdx, _ := readInt32(r)
		for j := int(startIdx); j < int(endIdx) && j < len(ctx.geometry); j++ {
			e := ctx.geometry[j]
			part.Cubes = append(part.Cubes, CubeData{
				Pos:        Vec3i{int(e.x), int(e.y), int(e.z)},
				Type:       CubeSolid,
				FaceColors: e.faceColors,
			})
		}
	}

	// sub-chunks (markers etc.)
	for {
		header, err := readChunkHeader(r)
		if err != nil || header == nil {
			break
		}
		data, _ := readChunkBody(r, header)
		if data == nil {
			continue
		}
		if header.name == "PRMK" {
			br := bytes.NewReader(data)
			mc, _ := readInt32(br)
			for i := int32(0); i < mc; i++ {
				mx, _ := readByte(br)
				my, _ := readByte(br)
				mz, _ := readByte(br)
				mm, _ := readByte(br)
				part.Markers = append(part.Markers, Marker{
					X: int(int8(mx)), Y: int(int8(my)), Z: int(int8(mz)), Meta: int(mm),
				})
			}
		}
	}

	skin.Parts = append(skin.Parts, part)
}

func (ctx *v20Context) readPaintChunk(r io.Reader, skin *SkinFile) {
	readVarInt(r) // options
	totalW, _ := readVarInt(r)
	totalH, _ := readVarInt(r)

	paintData := make([]int32, totalW*totalH)

	count, _ := readVarInt(r)
	for i := 0; i < count; i++ {
		readVarInt(r) // width
		readVarInt(r) // height
		pixelCount, _ := readVarInt(r)
		for j := 0; j < pixelCount; j++ {
			colorIdx, _ := readVarInt(r)
			if j < len(paintData) && colorIdx >= 0 && colorIdx < len(ctx.palette) {
				c := ctx.palette[colorIdx]
				paintData[j] = int32(uint32(0xFF)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B))
			}
		}
	}
	skin.PaintData = paintData
}
