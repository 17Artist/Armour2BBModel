// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"fmt"

	"github.com/17Artist/Armour2BBModel/bbmodel"
	"github.com/17Artist/Armour2BBModel/skin"
)

// Convert 将 AW SkinFile 转换为 BBModel，按模板骨骼输出
func Convert(sf *skin.SkinFile) (*bbmodel.Model, error) {
	name := sf.CustomName()
	if name == "" {
		name = sf.SkinType
	}

	model := bbmodel.NewModel(name)

	bones := SkinTypeBones[sf.SkinType]
	if bones == nil {
		bones = SkinTypeBones["armourers:outfit"]
	}

	bonePartMap := map[string][]*skin.PartData{}
	for _, part := range sf.Parts {
		bone := PartToBone[part.PartType]
		if bone == nil {
			continue
		}
		bonePartMap[bone.Name] = append(bonePartMap[bone.Name], part)
	}

	atlas := NewTextureAtlas()

	type boneWork struct {
		bone        *Bone
		boxes       []MergedBox
		faceRegions [][6]int
	}
	var works []boneWork

	for _, bone := range bones {
		parts := bonePartMap[bone.Name]
		if len(parts) == 0 {
			continue
		}

		// 合并同骨骼下所有 part 的 cubes
		var allCubes []skin.CubeData
		for _, p := range parts {
			allCubes = append(allCubes, p.Cubes...)
		}
		if len(allCubes) == 0 {
			continue
		}

		vis := ComputeFaceCulling(allCubes)
		boxes := GreedyMerge(allCubes, vis)

		bw := boneWork{bone: bone, boxes: boxes}
		bw.faceRegions = make([][6]int, len(boxes))
		for i, box := range boxes {
			for f := 0; f < 6; f++ {
				if box.Faces[f].Visible && box.Faces[f].Width > 0 && box.Faces[f].Height > 0 {
					bw.faceRegions[i][f] = atlas.AddFace(box.Faces[f], f)
				} else {
					bw.faceRegions[i][f] = -1
				}
			}
		}
		works = append(works, bw)
	}

	atlas.Pack()
	if atlas.RegionCount() > 0 {
		src, err := atlas.GenerateBase64PNG()
		if err != nil {
			return nil, fmt.Errorf("generating texture: %w", err)
		}
		w, h := atlas.Width(), atlas.Height()
		model.Resolution = &bbmodel.Resolution{Width: w, Height: h}
		model.Textures = append(model.Textures, bbmodel.Texture{
			Name: "palette", ID: "0",
			Width: w, Height: h, UVWidth: w, UVHeight: h,
			RenderMode: "default", RenderSides: "auto",
			Visible: true, Internal: true, Saved: true,
			UUID: bbmodel.NewUUID(), Source: src,
		})
	} else {
		model.Resolution = &bbmodel.Resolution{Width: 16, Height: 16}
	}

	groupUUIDs := map[string]string{}

	for wi, bw := range works {
		group := bbmodel.Group{
			UUID:       bbmodel.NewUUID(),
			Export:     true,
			Origin:     bw.bone.Origin,
			Rotation:   [3]float32{0, 0, 0},
			Color:      wi % 8,
			Name:       bw.bone.Name,
			Children:   []any{},
			Shade:      true,
			Visibility: true,
			IsOpen:     true,
		}
		groupUUIDs[bw.bone.Name] = group.UUID

		node := bbmodel.OutlinerNode{
			UUID:     group.UUID,
			IsOpen:   true,
			Children: []any{},
		}

		texID := 0
		for i, box := range bw.boxes {
			from, to := MapBox(box.MinX, box.MinY, box.MinZ, box.MaxX, box.MaxY, box.MaxZ, bw.bone.AWOrigin)

			elem := bbmodel.Element{
				Name:        fmt.Sprintf("%s_%d", bw.bone.Name, i),
				BoxUV:       false,
				RenderOrder: "default",
				From:        from,
				To:          to,
				Origin:      bw.bone.Origin,
				UUID:        bbmodel.NewUUID(),
				Type:        "cube",
				Faces:       make(map[string]bbmodel.Face, 6),
			}

			if box.CubeType.IsGlowing() {
				v := 15
				elem.LightEmission = &v
			}
			if box.CubeType.IsGlass() {
				v := false
				elem.Shade = &v
			}

			for awFace := 0; awFace < 6; awFace++ {
				bbFaceName := AWFaceToBBName[awFace]
				regionIdx := bw.faceRegions[i][awFace]
				if regionIdx >= 0 {
					uv := atlas.GetUV(regionIdx)
					elem.Faces[bbFaceName] = bbmodel.Face{UV: uv, Texture: &texID}
				} else {
					elem.Faces[bbFaceName] = bbmodel.Face{UV: [4]float32{0, 0, 0, 0}}
				}
			}

			model.Elements = append(model.Elements, elem)
			node.Children = append(node.Children, elem.UUID)
		}

		model.Groups = append(model.Groups, group)
		model.Outliner = append(model.Outliner, node)
	}

	if sf.SkinType == "armourers:wings" || sf.SkinType == "armourers:outfit" {
		GenerateWingAnimation(model, sf, groupUUIDs)
	}

	return model, nil
}
