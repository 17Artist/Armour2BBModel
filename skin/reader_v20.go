// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
)

type v20Context struct {
	palette         []PaintColor
	colorIndexBytes int
	geometry        []CubeData
	decodedBytes    int
}

type namedChunk struct {
	name string
	id   int
	data []byte
}

// Chunk lengths include the length field itself (10-byte header, or 14 with a part ID).
// SKPR contains the entire part table; selectors are (start, count), not (start, end).
func (ctx *v20Context) readChunks(r *bytes.Reader, partChunks bool) ([]namedChunk, error) {
	var chunks []namedChunk
	for {
		length, err := readInt32(r)
		if err != nil {
			return nil, fmt.Errorf("reading chunk length: %w", err)
		}
		if length == 0 {
			return chunks, nil
		}
		headerSize := 10
		if partChunks {
			headerSize = 14
		}
		if int(length) < headerSize || int(length) > MaxInputBytes {
			return nil, fmt.Errorf("invalid chunk length: %d", length)
		}
		var name [4]byte
		if _, err = io.ReadFull(r, name[:]); err != nil {
			return nil, err
		}
		flags, err := readInt16(r)
		if err != nil {
			return nil, err
		}
		id := 0
		if partChunks {
			v, e := readInt32(r)
			if e != nil {
				return nil, e
			}
			id = int(v)
		}
		size := int(length) - headerSize
		if size > r.Len() {
			return nil, io.ErrUnexpectedEOF
		}
		raw := make([]byte, size)
		io.ReadFull(r, raw)
		if flags&1 != 0 {
			return nil, fmt.Errorf("encrypted %s chunk is unsupported; export an unencrypted skin", string(name[:]))
		}
		if flags & ^int16(3) != 0 {
			return nil, fmt.Errorf("unsupported %s chunk flags: %d", string(name[:]), flags)
		}
		if flags&2 != 0 {
			gr, e := gzip.NewReader(bytes.NewReader(raw))
			if e != nil {
				return nil, fmt.Errorf("opening %s gzip chunk: %w", string(name[:]), e)
			}
			remaining := maxDecodedBytes - ctx.decodedBytes
			raw, e = io.ReadAll(io.LimitReader(gr, int64(remaining)+1))
			gr.Close()
			if e != nil {
				return nil, fmt.Errorf("decompressing %s chunk: %w", string(name[:]), e)
			}
		}
		ctx.decodedBytes += len(raw)
		if ctx.decodedBytes > maxDecodedBytes {
			return nil, fmt.Errorf("decompressed chunk data exceeds limit")
		}
		chunks = append(chunks, namedChunk{string(name[:]), id, raw})
		if len(chunks) > maxProperties {
			return nil, fmt.Errorf("too many chunks")
		}
	}
}

type valueReader struct {
	*bytes.Reader
	err error
}

func newValueReader(data []byte) *valueReader { return &valueReader{Reader: bytes.NewReader(data)} }
func (r *valueReader) b() byte {
	if r.err != nil {
		return 0
	}
	v, e := readByte(r.Reader)
	r.err = e
	return v
}
func (r *valueReader) i32() int {
	if r.err != nil {
		return 0
	}
	v, e := readInt32(r.Reader)
	r.err = e
	return int(v)
}
func (r *valueReader) u16() uint16 {
	if r.err != nil {
		return 0
	}
	v, e := readInt16(r.Reader)
	r.err = e
	return uint16(v)
}
func (r *valueReader) vi() int {
	if r.err != nil {
		return 0
	}
	v, e := readVarInt(r.Reader)
	r.err = e
	return v
}
func (r *valueReader) str() string {
	if r.err != nil {
		return ""
	}
	v, e := readString(r.Reader)
	r.err = e
	return v
}
func (r *valueReader) f32() float32 {
	if r.err != nil {
		return 0
	}
	v, e := readFloat32(r.Reader)
	r.err = e
	if e == nil && (math.IsNaN(float64(v)) || math.IsInf(float64(v), 0)) {
		r.err = fmt.Errorf("non-finite transform")
	}
	return v
}
func (r *valueReader) count(name string, limit int) int {
	v := r.vi()
	if r.err == nil {
		r.err = checkCount(name, v, limit)
	}
	return v
}
func (r *valueReader) done() error {
	if r.err != nil {
		return r.err
	}
	if r.Len() != 0 {
		return fmt.Errorf("%d unread bytes", r.Len())
	}
	return nil
}

func readV20(input io.Reader, fileVersion int) (*SkinFile, error) {
	data, e := io.ReadAll(input)
	if e != nil {
		return nil, e
	}
	r := newValueReader(data)
	r.i32()
	r.i32()
	sf := &SkinFile{FileVersion: fileVersion, SkinType: r.str(), Properties: map[string]string{}}
	if r.err != nil {
		return nil, fmt.Errorf("reading skin header: %w", r.err)
	}
	ctx := &v20Context{}
	chunks, e := ctx.readChunks(r.Reader, false)
	if e != nil {
		return nil, e
	}
	if r.Len() != 4 {
		return nil, fmt.Errorf("missing or invalid skin footer")
	}
	r.i32()
	if e = r.done(); e != nil {
		return nil, fmt.Errorf("skin footer: %w", e)
	}
	seen := map[string]bool{}
	for _, c := range chunks {
		if c.name == "PALE" || c.name == "CCBO" || c.name == "SKPR" || c.name == "PPTS" || c.name == "PADT" {
			if seen[c.name] {
				return nil, fmt.Errorf("duplicate %s chunk", c.name)
			}
			seen[c.name] = true
		}
	}
	for _, c := range chunks {
		if c.name == "PALE" {
			if e = ctx.readPaletteChunk(c.data); e != nil {
				return nil, fmt.Errorf("PALE: %w", e)
			}
		}
	}
	for _, c := range chunks {
		if c.name == "CCBO" {
			if e = ctx.readGeometryChunk(c.data); e != nil {
				return nil, fmt.Errorf("CCBO: %w", e)
			}
		}
	}
	for _, c := range chunks {
		switch c.name {
		case "PPTS":
			pr := bytes.NewReader(c.data)
			sf.Properties, e = readProperties(pr)
			if e == nil && pr.Len() != 0 {
				e = fmt.Errorf("%d unread property bytes", pr.Len())
			}
		case "SKPR":
			e = ctx.readPartChunk(c.data, sf)
		case "PADT":
			e = ctx.readPaintChunk(c.data, sf)
		case "ANIM":
			ar := newValueReader(c.data)
			count := ar.count("animations", maxParts)
			if ar.err != nil {
				e = ar.err
			} else if count > 0 {
				sf.Warnings = append(sf.Warnings, "源动画未导出，已保留静态姿态")
			}
		}
		if e != nil {
			return nil, fmt.Errorf("%s: %w", c.name, e)
		}
	}
	if !seen["SKPR"] {
		return nil, fmt.Errorf("missing SKPR part table")
	}
	return sf, nil
}

func (ctx *v20Context) readPaletteChunk(data []byte) error {
	r := newValueReader(data)
	options := r.vi()
	r.vi()
	ctx.colorIndexBytes = options & 0x0f
	if ctx.colorIndexBytes < 1 || ctx.colorIndexBytes > 4 {
		return fmt.Errorf("invalid palette index width: %d", ctx.colorIndexBytes)
	}
	for r.err == nil {
		count := r.count("palette colors", maxCubes)
		if count == 0 {
			break
		}
		paintType := r.b()
		usedBytes := int(r.b())
		if usedBytes != 3 && usedBytes != 4 {
			return fmt.Errorf("texture palettes are unsupported; use painted voxel skins")
		}
		if len(ctx.palette)+count > maxCubes {
			return fmt.Errorf("palette exceeds limit")
		}
		for i := 0; i < count && r.err == nil; i++ {
			var v uint32
			for j := 0; j < usedBytes; j++ {
				v = v<<8 | uint32(r.b())
			}
			ctx.palette = append(ctx.palette, PaintColor{R: byte(v >> 16), G: byte(v >> 8), B: byte(v), PaintType: paintType})
		}
	}
	return r.done()
}

func (ctx *v20Context) readGeometryChunk(data []byte) error {
	r := newValueReader(data)
	for r.err == nil {
		count := r.count("geometry", maxCubes)
		if count == 0 {
			break
		}
		geometryType := r.vi()
		options := r.vi()
		if geometryType < 0 || geometryType > 3 {
			return fmt.Errorf("geometry type %d uses imported cubes/meshes, which are unsupported; use painted voxel skins", geometryType)
		}
		faceCount := options & 0x0f
		if options != faceCount || faceCount < 1 || faceCount > 6 {
			return fmt.Errorf("invalid voxel geometry options: %d", options)
		}
		if ctx.colorIndexBytes == 0 {
			return fmt.Errorf("geometry requires a palette")
		}
		if len(ctx.geometry)+count > maxCubes {
			return fmt.Errorf("geometry exceeds limit")
		}
		for i := 0; i < count && r.err == nil; i++ {
			cube := CubeData{Pos: Vec3i{int(int8(r.b())), int(int8(r.b())), int(int8(r.b()))}, Type: CubeType(geometryType)}
			faceSet := byte(0)
			for j := 0; j < faceCount && r.err == nil; j++ {
				mask := r.b()
				index := uint32(0)
				for k := 0; k < ctx.colorIndexBytes; k++ {
					index = index<<8 | uint32(r.b())
				}
				if r.err != nil {
					break
				}
				if mask == 0 || mask&0xc0 != 0 || faceSet&mask != 0 {
					return fmt.Errorf("invalid or duplicate voxel face mask: %d", mask)
				}
				if uint64(index) >= uint64(len(ctx.palette)) {
					return fmt.Errorf("palette reference %d out of range", index)
				}
				faceSet |= mask
				for f := 0; f < 6; f++ {
					if mask&(1<<f) != 0 {
						cube.FaceColors[f] = ctx.palette[index]
					}
				}
			}
			if r.err == nil && faceSet != 0x3f {
				return fmt.Errorf("voxel face mask does not cover all six faces")
			}
			ctx.geometry = append(ctx.geometry, cube)
		}
	}
	return r.done()
}

func readPartTransform(r *valueReader, p *PartData) {
	flags := r.b()
	if flags == 0x10 {
		return
	}
	defaults := []float32{0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0}
	if flags == 0x20 {
		defaults = []float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0}
	}
	if flags != 0x20 && flags != 0x40 {
		r.err = fmt.Errorf("unsupported transform flags: %d", flags)
		return
	}
	mask := r.u16()
	for i := range defaults {
		if mask&(1<<i) == 0 {
			defaults[i] = r.f32()
		}
	}
	if flags == 0x20 {
		var m [16]float32
		copy(m[:], defaults)
		p.Matrix = &m
		return
	}
	p.TranslateX, p.TranslateY, p.TranslateZ = defaults[0], defaults[1], defaults[2]
	p.RotationX, p.RotationY, p.RotationZ = defaults[3], defaults[4], defaults[5]
	p.ScaleX, p.ScaleY, p.ScaleZ = defaults[6], defaults[7], defaults[8]
	p.OffsetX, p.OffsetY, p.OffsetZ = defaults[9], defaults[10], defaults[11]
	p.PivotX, p.PivotY, p.PivotZ = defaults[12], defaults[13], defaults[14]
}

func (ctx *v20Context) readPartChunk(data []byte, sf *SkinFile) error {
	r := newValueReader(data)
	count := r.count("parts", maxParts)
	parts := make(map[int]*PartData, count)
	parents := make(map[int]int, count)
	order := make([]int, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		id, parent := r.vi(), r.vi()
		p := &PartData{Name: r.str(), PartType: fixPartTypeName(r.str()), ScaleX: 1, ScaleY: 1, ScaleZ: 1}
		if id <= 0 || parts[id] != nil {
			return fmt.Errorf("invalid or duplicate part ID: %d", id)
		}
		readPartTransform(r, p)
		selectors := r.count("geometry selectors", maxParts)
		for j := 0; j < selectors && r.err == nil; j++ {
			start, size := r.i32(), r.i32()
			if r.err != nil {
				break
			}
			if start < 0 || size < 0 || start > len(ctx.geometry) || size > len(ctx.geometry)-start {
				return fmt.Errorf("part %d geometry range (%d, %d) out of bounds", id, start, size)
			}
			if len(p.Cubes)+size > maxCubes {
				return fmt.Errorf("part geometry exceeds limit")
			}
			p.Cubes = append(p.Cubes, ctx.geometry[start:start+size]...)
		}
		parts[id] = p
		parents[id] = parent
		order = append(order, id)
	}
	if r.err != nil {
		return r.err
	}
	subchunks, e := ctx.readChunks(r.Reader, true)
	if e != nil {
		return e
	}
	if e = r.done(); e != nil {
		return e
	}
	seenProperties := map[int]bool{}
	for _, c := range subchunks {
		p := parts[c.id]
		if p == nil {
			return fmt.Errorf("subchunk references missing part %d", c.id)
		}
		if c.name == "PRMK" {
			mr := newValueReader(c.data)
			count := mr.i32()
			if e = checkCount("markers", count, maxParts); e != nil {
				return e
			}
			for i := 0; i < count; i++ {
				p.Markers = append(p.Markers, Marker{int(int8(mr.b())), int(int8(mr.b())), int(int8(mr.b())), int(mr.b())})
			}
			if e = mr.done(); e != nil {
				return e
			}
		} else if c.name == "PPTS" {
			if seenProperties[c.id] {
				return fmt.Errorf("duplicate part %d PPTS chunk", c.id)
			}
			seenProperties[c.id] = true
			pr := bytes.NewReader(c.data)
			p.Properties, e = readProperties(pr)
			if e != nil {
				return fmt.Errorf("part %d properties: %w", c.id, e)
			}
			if pr.Len() != 0 {
				return fmt.Errorf("part %d properties contain %d unread bytes", c.id, pr.Len())
			}
		}
	}
	state := make(map[int]byte, count)
	var visit func(int) error
	visit = func(id int) error {
		if state[id] == 1 {
			return fmt.Errorf("cyclic part hierarchy at %d", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		parent := parents[id]
		if parent != 0 {
			if parts[parent] == nil {
				return fmt.Errorf("part %d references missing parent %d", id, parent)
			}
			if e := visit(parent); e != nil {
				return e
			}
		}
		state[id] = 2
		return nil
	}
	for _, id := range order {
		if e = visit(id); e != nil {
			return e
		}
	}
	for _, id := range order {
		p := parts[id]
		if parent := parents[id]; parent != 0 {
			parts[parent].Children = append(parts[parent].Children, p)
		} else {
			sf.Parts = append(sf.Parts, p)
		}
	}
	return nil
}

func (ctx *v20Context) readPaintChunk(data []byte, sf *SkinFile) error {
	r := newValueReader(data)
	r.i32()
	w, h := r.vi(), r.vi()
	if w < 0 || h < 0 || w > 4096 || h > 4096 || w*h > maxCubes {
		return fmt.Errorf("invalid paint dimensions %dx%d", w, h)
	}
	sf.PaintData = make([]int32, w*h)
	for i := range sf.PaintData {
		sf.PaintData[i] = int32(r.i32())
	}
	return r.done()
}
