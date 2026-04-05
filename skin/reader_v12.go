// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"fmt"
	"io"
)

const textureOldSize = 32 * 64

func readV12(r io.Reader, fileVersion int) (*SkinFile, error) {
	skin := &SkinFile{FileVersion: fileVersion}

	// v12+ 有 header tags
	if fileVersion > 12 {
		readString(r) // AW-SKIN-START
		readString(r) // PROPS-START
	}

	props, err := readPropsV12(r, fileVersion)
	if err != nil {
		return nil, fmt.Errorf("reading properties: %w", err)
	}
	skin.Properties = props

	if fileVersion > 12 {
		readString(r) // PROPS-END
		readString(r) // TYPE-START
	}

	skinType, err := readSkinTypeV12(r, fileVersion)
	if err != nil {
		return nil, fmt.Errorf("reading skin type: %w", err)
	}
	skin.SkinType = skinType

	if fileVersion > 12 {
		readString(r) // TYPE-END
		readString(r) // PAINT-START
	}

	if fileVersion > 7 {
		hasPaint, err := readBool(r)
		if err != nil {
			return nil, err
		}
		if hasPaint {
			paintData := make([]int32, textureOldSize)
			for i := range paintData {
				paintData[i], err = readInt32(r)
				if err != nil {
					return nil, err
				}
			}
			skin.PaintData = paintData
		}
	}

	if fileVersion > 12 {
		readString(r) // PAINT-END
	}

	partCountByte, err := readByte(r)
	if err != nil {
		return nil, err
	}
	partCount := int(partCountByte)

	for i := 0; i < partCount; i++ {
		if fileVersion > 12 {
			readString(r) // PART-START
		}
		part, err := readPartV12(r, fileVersion)
		if err != nil {
			return nil, fmt.Errorf("reading part %d: %w", i, err)
		}
		skin.Parts = append(skin.Parts, part)
		if fileVersion > 12 {
			readString(r) // PART-END
		}
	}

	if fileVersion > 12 {
		readString(r) // AW-SKIN-END
	}

	return skin, nil
}

func readPropsV12(r io.Reader, version int) (map[string]string, error) {
	if version < 12 {
		author, err := readString(r)
		if err != nil {
			return nil, err
		}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		props := map[string]string{
			"authorName": author,
			"customName": name,
		}
		if version >= 4 {
			tags, err := readString(r)
			if err != nil {
				return nil, err
			}
			if tags != "" {
				props["tags"] = tags
			}
		}
		return props, nil
	}
	return readProperties(r)
}

func readSkinTypeV12(r io.Reader, version int) (string, error) {
	if version < 5 {
		b, err := readByte(r)
		if err != nil {
			return "", err
		}
		return skinTypeByLegacyID(int(b) - 1), nil
	}
	return readString(r)
}

func readPartV12(r io.Reader, version int) (*PartData, error) {
	regName, err := readString(r)
	if err != nil {
		return nil, err
	}

	part := &PartData{
		PartType: fixPartTypeName(regName),
		ScaleX:   1, ScaleY: 1, ScaleZ: 1,
	}

	cubeCount, err := readInt32(r)
	if err != nil {
		return nil, err
	}

	if version >= 10 {
		stride := 4 + 4*6 // 28 bytes per cube
		data := make([]byte, int(cubeCount)*stride)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		for i := int32(0); i < cubeCount; i++ {
			off := int(i) * stride
			cube := parseCubeBytes(data, off, version)
			part.Cubes = append(part.Cubes, cube)
		}
	} else {
		for i := int32(0); i < cubeCount; i++ {
			cube, err := readLegacyCube(r, version)
			if err != nil {
				return nil, err
			}
			part.Cubes = append(part.Cubes, cube)
		}
	}

	markerCount, err := readInt32(r)
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < markerCount; i++ {
		x, _ := readByte(r)
		y, _ := readByte(r)
		z, _ := readByte(r)
		meta, err := readByte(r)
		if err != nil {
			return nil, err
		}
		part.Markers = append(part.Markers, Marker{
			X: int(int8(x)), Y: int(int8(y)), Z: int(int8(z)), Meta: int(meta),
		})
	}

	return part, nil
}

func parseCubeBytes(data []byte, off int, version int) CubeData {
	cube := CubeData{
		Pos:  Vec3i{int(int8(data[off+1])), int(int8(data[off+2])), int(int8(data[off+3]))},
		Type: CubeType(data[off] & 0x0F),
	}
	for side := 0; side < 6; side++ {
		base := off + 4 + side*4
		t := data[base+3]
		if version < 11 {
			t = 255
		}
		cube.FaceColors[side] = PaintColor{
			R: data[base], G: data[base+1], B: data[base+2], PaintType: t,
		}
	}
	return cube
}

func readLegacyCube(r io.Reader, version int) (CubeData, error) {
	id, err := readByte(r)
	if err != nil {
		return CubeData{}, err
	}
	x, _ := readByte(r)
	y, _ := readByte(r)
	z, err := readByte(r)
	if err != nil {
		return CubeData{}, err
	}

	cube := CubeData{
		Pos:  Vec3i{int(int8(x)), int(int8(y)), int(int8(z))},
		Type: CubeType(id & 0x0F),
	}

	for side := 0; side < 6; side++ {
		cr, _ := readByte(r)
		cg, _ := readByte(r)
		cb, err := readByte(r)
		if err != nil {
			return CubeData{}, err
		}
		cube.FaceColors[side] = PaintColor{R: cr, G: cg, B: cb, PaintType: 255}
	}

	return cube, nil
}
