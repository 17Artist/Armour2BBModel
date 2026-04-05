// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"fmt"
	"io"
)

func readV13(r io.Reader, fileVersion int) (*SkinFile, error) {
	skin := &SkinFile{FileVersion: fileVersion}

	// header
	if h, _ := readString(r); h != "AW-SKIN-START" {
		return nil, fmt.Errorf("invalid skin header: %s", h)
	}

	// properties
	if h, _ := readString(r); h != "PROPS-START" {
		return nil, fmt.Errorf("invalid props header: %s", h)
	}
	props, err := readProperties(r)
	if err != nil {
		return nil, err
	}
	skin.Properties = props
	if h, _ := readString(r); h != "PROPS-END" {
		return nil, fmt.Errorf("invalid props footer: %s", h)
	}

	// type
	if h, _ := readString(r); h != "TYPE-START" {
		return nil, fmt.Errorf("invalid type header: %s", h)
	}
	regName, err := readString(r)
	if err != nil {
		return nil, err
	}
	skin.SkinType = regName
	if h, _ := readString(r); h != "TYPE-END" {
		return nil, fmt.Errorf("invalid type footer: %s", h)
	}

	// paint data
	if h, _ := readString(r); h != "PAINT-START" {
		return nil, fmt.Errorf("invalid paint header: %s", h)
	}
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
	if h, _ := readString(r); h != "PAINT-END" {
		return nil, fmt.Errorf("invalid paint footer: %s", h)
	}

	// parts
	partCountByte, err := readByte(r)
	if err != nil {
		return nil, err
	}
	for i := 0; i < int(partCountByte); i++ {
		if h, _ := readString(r); h != "PART-START" {
			return nil, fmt.Errorf("invalid part header: %s", h)
		}
		part, err := readPartV13(r)
		if err != nil {
			return nil, fmt.Errorf("reading part %d: %w", i, err)
		}
		skin.Parts = append(skin.Parts, part)
		if h, _ := readString(r); h != "PART-END" {
			return nil, fmt.Errorf("invalid part footer: %s", h)
		}
	}

	if h, _ := readString(r); h != "AW-SKIN-END" {
		return nil, fmt.Errorf("invalid skin footer: %s", h)
	}

	return skin, nil
}

func readPartV13(r io.Reader) (*PartData, error) {
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

	stride := 4 + 4*6
	data := make([]byte, int(cubeCount)*stride)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	for i := int32(0); i < cubeCount; i++ {
		cube := parseCubeBytes(data, int(i)*stride, 13)
		part.Cubes = append(part.Cubes, cube)
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
