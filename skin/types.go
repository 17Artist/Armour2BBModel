// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

// Package skin 参照 Armourer's Workshop 开源代码实现 skin 文件格式的解析。
package skin

import (
	"fmt"
	"io"
	"math"
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
	return c.R == 0 && c.G == 0 && c.B == 0 && c.PaintType == 0
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

	// Transform
	TranslateX, TranslateY, TranslateZ float32
	RotationX, RotationY, RotationZ    float32
	ScaleX, ScaleY, ScaleZ             float32
}

func (p *PartData) HasTransform() bool {
	return p.TranslateX != 0 || p.TranslateY != 0 || p.TranslateZ != 0 ||
		p.RotationX != 0 || p.RotationY != 0 || p.RotationZ != 0 ||
		p.ScaleX != 1 || p.ScaleY != 1 || p.ScaleZ != 1
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
	return string(data), nil
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
	value := 0
	shift := 0
	for {
		b, err := readByte(r)
		if err != nil {
			return 0, err
		}
		value |= int(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	return value, nil
}

// readVarString 读取变长字符串
func readVarString(r io.Reader) (string, error) {
	length, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if length <= 0 {
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
	count, err := readInt32(r)
	if err != nil {
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
		value, err := readTypedValue(r, typeByte)
		if err != nil {
			return nil, fmt.Errorf("reading property value for key %q (type %d): %w", key, typeByte, err)
		}
		props[key] = value
	}
	return props, nil
}

// readTypedValue 根据类型字节读取对应类型的值，统一转为字符串
func readTypedValue(r io.Reader, typeByte byte) (string, error) {
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
		return fmt.Sprintf("%f", math.Float64frombits(bits)), nil
	case 3: // BOOLEAN
		b, err := readBool(r)
		return fmt.Sprintf("%t", b), err
	case 4: // LIST
		size, err := readInt32(r)
		if err != nil {
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
			if _, err := readTypedValue(r, elemType); err != nil {
				return "", err
			}
		}
		return "[]", nil
	case 5: // COMPOUND (嵌套属性)
		_, err := readProperties(r)
		return "{}", err
	case 6: // FLOAT
		v, err := readFloat32(r)
		return fmt.Sprintf("%f", v), err
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
