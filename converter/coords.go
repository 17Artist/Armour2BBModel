// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

// MapBox 将 AW 坐标范围转为 BBModel 绝对坐标
// bone origin 作为坐标中心，X/Y 翻转，Z 不翻转
func MapBox(x1, y1, z1, x2, y2, z2 int, origin [3]float32) (from, to [3]float32) {
	from = [3]float32{
		-float32(x2) + origin[0],
		-float32(y2) + origin[1],
		float32(z1) + origin[2],
	}
	to = [3]float32{
		-float32(x1) + origin[0],
		-float32(y1) + origin[1],
		float32(z2) + origin[2],
	}
	return
}

// AWFaceToBBName AW 面索引 → BBModel 面名称
// X/Y 翻转后: AW down→BB up, AW up→BB down, AW west→BB east, AW east→BB west
var AWFaceToBBName = [6]string{"up", "down", "north", "south", "east", "west"}
