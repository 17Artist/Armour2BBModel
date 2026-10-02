// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

// Package skin 参照 Armourer's Workshop 开源代码实现 skin 文件格式的解析。
package skin

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// CubeType 方块类型
type CubeType byte

const (
	CubeSolid        CubeType = 0
	CubeGlowing      CubeType = 1
	CubeGlass        CubeType = 2
	CubeGlassGlowing CubeType = 3
)

func (c CubeType) IsGlowing() bool { return c == CubeGlowing || c == CubeGlassGlowing }
func (c CubeType) IsGlass() bool   { return c == CubeGlass || c == CubeGlassGlowing }

// PaintColor 颜色数据
type PaintColor struct {
	R, G, B   byte
	PaintType byte
}

var EmptyColor = PaintColor{}

func (c PaintColor) ToRGB() int {
	return int(c.R)<<16 | int(c.G)<<8 | int(c.B)
}

func (c PaintColor) IsEmpty() bool {
	return c.PaintType == 0
}

// Vec3i 整数三维向量
type Vec3i struct {
	X, Y, Z int
}

// Marker 标记点
type Marker struct {
	X, Y, Z int
	Meta    int
}

// CubeData 单个方块数据
type CubeData struct {
	Pos        Vec3i
	Type       CubeType
	FaceColors [6]PaintColor // down, up, north, south, west, east
}

// PartData 部件数据
type PartData struct {
	PartType string
	Name     string
	Cubes    []CubeData
	Markers  []Marker
	Children []*PartData
	// Properties contains this part's own settings, or the numbered settings of its
	// source outfit equipment. Parsed parts always have a non-nil map; children do
	// not inherit wing settings from their parent.
	Properties map[string]string
	// EquipmentIndex is the zero-based source outfit entry, not a visual body category.
	EquipmentIndex *int

	// Transform
	TranslateX, TranslateY, TranslateZ float32
	RotationX, RotationY, RotationZ    float32
	ScaleX, ScaleY, ScaleZ             float32
	OffsetX, OffsetY, OffsetZ          float32
	PivotX, PivotY, PivotZ             float32
	// Matrix contains a column-major affine transform when the source stores a matrix.
	Matrix *[16]float32
}

func (p *PartData) HasTransform() bool {
	return p.TranslateX != 0 || p.TranslateY != 0 || p.TranslateZ != 0 ||
		p.RotationX != 0 || p.RotationY != 0 || p.RotationZ != 0 ||
		p.ScaleX != 1 || p.ScaleY != 1 || p.ScaleZ != 1 ||
		p.OffsetX != 0 || p.OffsetY != 0 || p.OffsetZ != 0 ||
		p.PivotX != 0 || p.PivotY != 0 || p.PivotZ != 0 || p.Matrix != nil
}

// DisplayName 获取可读的部件名称
func (p *PartData) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.PartType != "" {
		return p.PartType
	}
	return "unknown"
}

// SkinFile 顶层数据
type SkinFile struct {
	FileVersion int
	SkinType    string
	Properties  map[string]string
	PaintData   []int32
	Parts       []*PartData
	Warnings    []string
}

func (s *SkinFile) CustomName() string {
	if s.Properties != nil {
		return s.Properties["customName"]
	}
	return ""
}

// FaceDirection 面方向常量
const (
	FaceDown  = 0 // Y-
	FaceUp    = 1 // Y+
	FaceNorth = 2 // Z-
	FaceSouth = 3 // Z+
	FaceWest  = 4 // X-
	FaceEast  = 5 // X+
)

var FaceNames = [6]string{"down", "up", "north", "south", "west", "east"}

// FaceOpposite 返回对面方向
func FaceOpposite(face int) int {
	switch face {
	case FaceDown:
		return FaceUp
	case FaceUp:
		return FaceDown
	case FaceNorth:
		return FaceSouth
	case FaceSouth:
		return FaceNorth
	case FaceWest:
		return FaceEast
	case FaceEast:
		return FaceWest
	}
	return -1
}

// FaceOffset 返回面方向对应的坐标偏移
func FaceOffset(face int) Vec3i {
	switch face {
	case FaceDown:
		return Vec3i{0, -1, 0}
	case FaceUp:
		return Vec3i{0, 1, 0}
	case FaceNorth:
		return Vec3i{0, 0, -1}
	case FaceSouth:
		return Vec3i{0, 0, 1}
	case FaceWest:
		return Vec3i{-1, 0, 0}
	case FaceEast:
		return Vec3i{1, 0, 0}
	}
	return Vec3i{}
}

// readString 读取 Java DataInputStream 格式的 UTF 字符串
func readString(r io.Reader) (string, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return "", err
	}
	length := int(buf[0])<<8 | int(buf[1])
	if length == 0 {
		return "", nil
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	// Older Java DataOutputStream.writeUTF exports use modified UTF-8 (NUL and surrogate pairs).
	// Current AW exports use ordinary UTF-8; valid UTF-8 always takes the direct path above.
	var units []uint16
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b < 0x80:
			units = append(units, uint16(b))
			i++
		case b&0xe0 == 0xc0 && i+1 < len(data) && data[i+1]&0xc0 == 0x80:
			v := uint16(b&0x1f)<<6 | uint16(data[i+1]&0x3f)
			if v < 0x80 && v != 0 {
				return "", fmt.Errorf("invalid string encoding")
			}
			units = append(units, v)
			i += 2
		case b&0xf0 == 0xe0 && i+2 < len(data) && data[i+1]&0xc0 == 0x80 && data[i+2]&0xc0 == 0x80:
			v := uint16(b&0x0f)<<12 | uint16(data[i+1]&0x3f)<<6 | uint16(data[i+2]&0x3f)
			if v < 0x800 {
				return "", fmt.Errorf("invalid string encoding")
			}
			units = append(units, v)
			i += 3
		default:
			return "", fmt.Errorf("invalid string encoding")
		}
	}
	return string(utf16.Decode(units)), nil
}

// readInt32 读取大端 int32
func readInt32(r io.Reader) (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return int32(buf[0])<<24 | int32(buf[1])<<16 | int32(buf[2])<<8 | int32(buf[3]), nil
}

// readInt16 读取大端 int16
func readInt16(r io.Reader) (int16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return int16(buf[0])<<8 | int16(buf[1]), nil
}

// readByte 读取单字节
func readByte(r io.Reader) (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// readBool 读取布尔值
func readBool(r io.Reader) (bool, error) {
	b, err := readByte(r)
	return b != 0, err
}

// readFloat32 读取大端 float32
func readFloat32(r io.Reader) (float32, error) {
	v, err := readInt32(r)
	if err != nil {
		return 0, err
	}
	return float32FromBits(uint32(v)), nil
}

func float32FromBits(b uint32) float32 {
	return math.Float32frombits(b)
}

// readVarInt 读取变长整数
func readVarInt(r io.Reader) (int, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		b, err := readByte(r)
		if err != nil {
			return 0, err
		}
		if i == 4 && b&0xF0 != 0 {
			return 0, fmt.Errorf("invalid 32-bit VarInt")
		}
		value |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return int(int32(value)), nil
		}
	}
	return 0, fmt.Errorf("VarInt exceeds five bytes")
}

// readVarString 读取变长字符串
func readVarString(r io.Reader) (string, error) {
	length, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 || length > maxStringBytes {
		return "", fmt.Errorf("invalid string length: %d", length)
	}
	if length == 0 {
		return "", nil
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

// readProperties 读取 AW 的类型化属性键值对
// 格式: count(4B) + [key(UTF) + type(1B) + value(typed)] * count
// type: 0=STRING, 1=INT, 2=DOUBLE, 3=BOOLEAN, 4=LIST, 5=COMPOUND, 6=FLOAT, 7=LONG
func readProperties(r io.Reader) (map[string]string, error) {
	return readPropertiesDepth(r, 0)
}

func readPropertiesDepth(r io.Reader, depth int) (map[string]string, error) {
	if depth > maxPropertyDepth {
		return nil, fmt.Errorf("property nesting exceeds %d", maxPropertyDepth)
	}
	count, err := readInt32(r)
	if err != nil {
		return nil, err
	}
	if err := checkCount("properties", int(count), maxProperties); err != nil {
		return nil, err
	}
	props := make(map[string]string, count)
	for i := int32(0); i < count; i++ {
		key, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("reading property key %d: %w", i, err)
		}
		typeByte, err := readByte(r)
		if err != nil {
			return nil, fmt.Errorf("reading property type for key %q: %w", key, err)
		}
		value, err := readTypedValueDepth(r, typeByte, depth)
		if err != nil {
			return nil, fmt.Errorf("reading property value for key %q (type %d): %w", key, typeByte, err)
		}
		props[key] = value
	}
	return props, nil
}

// readTypedValue 根据类型字节读取对应类型的值，统一转为字符串
func readTypedValue(r io.Reader, typeByte byte) (string, error) {
	return readTypedValueDepth(r, typeByte, 0)
}

func readTypedValueDepth(r io.Reader, typeByte byte, depth int) (string, error) {
	if depth > maxPropertyDepth {
		return "", fmt.Errorf("property nesting exceeds %d", maxPropertyDepth)
	}
	switch typeByte {
	case 0: // STRING
		s, err := readString(r)
		return s, err
	case 1: // INT
		v, err := readInt32(r)
		return fmt.Sprintf("%d", v), err
	case 2: // DOUBLE
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return "", err
		}
		bits := uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
			uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
		return strconv.FormatFloat(math.Float64frombits(bits), 'g', -1, 64), nil
	case 3: // BOOLEAN
		b, err := readBool(r)
		return fmt.Sprintf("%t", b), err
	case 4: // LIST
		size, err := readInt32(r)
		if err != nil {
			return "", err
		}
		if err := checkCount("list elements", int(size), maxProperties); err != nil {
			return "", err
		}
		if size == 0 {
			return "[]", nil
		}
		// 读取元素类型（只有第一个元素有类型标记）
		elemType, err := readByte(r)
		if err != nil {
			return "", err
		}
		for j := int32(0); j < size; j++ {
			if _, err := readTypedValueDepth(r, elemType, depth+1); err != nil {
				return "", err
			}
		}
		return "[]", nil
	case 5: // COMPOUND (嵌套属性)
		_, err := readPropertiesDepth(r, depth+1)
		return "{}", err
	case 6: // FLOAT
		v, err := readFloat32(r)
		return strconv.FormatFloat(float64(v), 'g', -1, 32), err
	case 7: // LONG
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return "", err
		}
		v := int64(buf[0])<<56 | int64(buf[1])<<48 | int64(buf[2])<<40 | int64(buf[3])<<32 |
			int64(buf[4])<<24 | int64(buf[5])<<16 | int64(buf[6])<<8 | int64(buf[7])
		return fmt.Sprintf("%d", v), nil
	default:
		return "", fmt.Errorf("unknown property type: %d", typeByte)
	}
}

// Limits keep malformed uploads bounded in both the native CLI and browser WASM.
const (
	MaxInputBytes    = 64 << 20
	maxDecodedBytes  = 128 << 20
	maxStringBytes   = 1 << 20
	maxCubes         = 1_000_000
	maxParts         = 4096
	maxProperties    = 16384
	maxPropertyDepth = 32
)

func checkCount(name string, count, limit int) error {
	if count < 0 || count > limit {
		return fmt.Errorf("invalid %s count %s (maximum %d)", name, strconv.Itoa(count), limit)
	}
	return nil
}

// skinTypeByLegacyID 旧版 ID 到类型名的映射
func skinTypeByLegacyID(id int) string {
	switch id {
	case 0:
		return "armourers:head"
	case 1:
		return "armourers:chest"
	case 2:
		return "armourers:legs"
	case 3:
		return "armourers:skirt"
	case 4:
		return "armourers:feet"
	case 5:
		return "armourers:sword"
	case 6:
		return "armourers:bow"
	case 7:
		return "armourers:arrow"
	default:
		return "armourers:unknown"
	}
}

// fixPartTypeName 修正旧版部件类型名
func fixPartTypeName(name string) string {
	switch name {
	case "armourers:skirt.base":
		return "armourers:legs.skirt"
	case "armourers:bow.base":
		return "armourers:bow.frame1"
	case "armourers:arrow.base":
		return "armourers:bow.arrow"
	default:
		return name
	}
}
