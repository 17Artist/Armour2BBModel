// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package converter

import (
	"fmt"

	"github.com/17Artist/Armour2BBModel/bbmodel"
	"github.com/17Artist/Armour2BBModel/skin"
)

// Convert preserves source part boundaries and hierarchy. Each part is classified by its
// registry ID. Local transforms, duplicate positions and different attachments never share a merge.
type Options struct{ KeepCoincidentFaces bool }

type partWork struct {
	part     *skin.PartData
	bone     *Bone // source coordinate origin; separate from the target animation joint
	matrix   affine
	group    bbmodel.Group
	cubes    []skin.CubeData
	boxes    []MergedBox
	regions  [][6]int
	children []*partWork
	wing     *wingBinding
	order    int
	target   string
	moving   bool
}

func Convert(sf *skin.SkinFile) (*bbmodel.Model, error) {
	return ConvertWithOptions(sf, Options{})
}

// KeepCoincidentFaces retains individual attachment surfaces for independent editing.
func ConvertWithOptions(sf *skin.SkinFile, options Options) (*bbmodel.Model, error) {
	if sf == nil {
		return nil, fmt.Errorf("nil skin")
	}
	name := sf.CustomName()
	if name == "" {
		name = sf.SkinType
	}
	model := bbmodel.NewModel(name)
	atlas := NewTextureAtlas()

	var wings []*wingBinding
	path := map[*skin.PartData]bool{}
	totalCubes := 0
	workCount := 0
	var build func(*skin.PartData, *Bone, affine, int, int, bool, string) (*partWork, error)
	build = func(p *skin.PartData, parentBone *Bone, parentMatrix affine, depth, sourceIndex int, parentMoving bool, parentTarget string) (*partWork, error) {
		if p == nil {
			return nil, fmt.Errorf("nil source part")
		}
		if depth > 128 || path[p] {
			return nil, fmt.Errorf("invalid or cyclic source part hierarchy")
		}
		workCount++
		if workCount > 4096 {
			return nil, fmt.Errorf("source part limit exceeded")
		}
		path[p] = true
		defer delete(path, p)
		totalCubes += len(p.Cubes)
		if totalCubes > 1_000_000 {
			return nil, fmt.Errorf("source voxel limit exceeded")
		}
		bone := resolvePartBone(p.PartType, parentBone)
		local, e := partMatrix(p)
		if e != nil {
			return nil, fmt.Errorf("part %q: %w", p.DisplayName(), e)
		}
		wing := wingForPart(p, sf, parentMatrix)
		if wing != nil {
			local = multiply(wing.localMatrix(), local)
		}
		matrix := multiply(parentMatrix, local)
		// Editable source names never impersonate an ArcartX host joint.
		groupName := fmt.Sprintf("Source_%s_%d", bone.Name, workCount)
		target := targetRootFor(bone.Name)
		if parentMoving {
			target = parentTarget
		}
		group := bbmodel.Group{
			UUID: bbmodel.NewUUID(), Export: true, Origin: bone.Origin, Color: (workCount - 1) % 8,
			Name: groupName, SourcePart: p.PartType, SourceName: p.Name, SourceEquipment: p.EquipmentIndex, Attachment: bone.Name, Children: []any{}, Shade: true, Visibility: true, IsOpen: true,
			SourceIndex: &sourceIndex, TargetBone: target, CostumeSlot: slotForTarget(target),
		}
		if wing != nil {
			group.Origin = wing.origin
			wing.groupUUID, wing.name = group.UUID, groupName
			wings = append(wings, wing)
		}
		w := &partWork{part: p, bone: bone, matrix: matrix, group: group, wing: wing, order: workCount, target: target, cubes: p.Cubes, moving: parentMoving || (wing != nil && wing.params.MinAngle != wing.params.MaxAngle)}
		for _, cube := range p.Cubes {
			if cube.Type > skin.CubeGlassGlowing {
				return nil, fmt.Errorf("part %q contains unsupported cube type %d", p.DisplayName(), cube.Type)
			}
		}
		for _, child := range p.Children {
			cw, e := build(child, bone, matrix, depth+1, sourceIndex, w.moving, w.target)
			if e != nil {
				return nil, e
			}
			w.children = append(w.children, cw)
		}
		return w, nil
	}
	var works []*partWork
	for sourceIndex, part := range sf.Parts {
		w, e := build(part, nil, identity(), 0, sourceIndex, false, "")
		if e != nil {
			return nil, e
		}
		works = append(works, w)
	}
	if totalCubes == 0 {
		return nil, fmt.Errorf("skin contains no voxel geometry; painted player textures and imported models require a voxel skin")
	}
	works = classifyTargetWorks(works)
	// Each target joint owns one root. Geometry is merged only within its source
	// fragment, and coincident faces only within the same rigid target joint.
	surfaces := map[string][]staticSurfaceWork{}
	var prepare func(*partWork) error
	prepare = func(w *partWork) error {
		var layers [][]skin.CubeData
		counts := map[skin.Vec3i]int{}
		for _, cube := range w.cubes {
			layer := counts[cube.Pos]
			counts[cube.Pos]++
			if layer == len(layers) {
				layers = append(layers, nil)
			}
			layers[layer] = append(layers[layer], cube)
		}
		for _, cubes := range layers {
			w.boxes = append(w.boxes, GreedyMerge(cubes, ComputeFaceCulling(cubes))...)
		}
		w.regions = make([][6]int, len(w.boxes))
		for _, box := range w.boxes {
			if _, _, _, _, err := mapTransformedBox(box, w.matrix, w.bone.AWOrigin); err != nil {
				return fmt.Errorf("part %q: %w", w.part.DisplayName(), err)
			}
		}
		if !w.moving {
			surfaces[w.target] = append(surfaces[w.target], staticSurfaceWork{w.boxes, w.matrix, w.bone.AWOrigin, w.order})
		}
		for _, child := range w.children {
			if err := prepare(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, w := range works {
		if err := prepare(w); err != nil {
			return nil, err
		}
	}
	if !options.KeepCoincidentFaces {
		for _, jointSurfaces := range surfaces {
			model.SurfaceDeduplicatedTexels += normalizeStaticSurfaces(jointSurfaces)
		}
	}
	var allocateFaces func(*partWork)
	allocateFaces = func(w *partWork) {
		for i, box := range w.boxes {
			for f, face := range box.Faces {
				w.regions[i][f] = -1
				if face.Visible && face.Width > 0 && face.Height > 0 {
					w.regions[i][f] = atlas.AddFace(face, f)
				}
			}
		}
		for _, child := range w.children {
			allocateFaces(child)
		}
	}
	for _, w := range works {
		allocateFaces(w)
	}

	atlas.Pack()
	if atlas.RegionCount() > 0 {
		src, e := atlas.GenerateBase64PNG()
		if e != nil {
			return nil, fmt.Errorf("generating texture: %w", e)
		}
		width, height := atlas.Width(), atlas.Height()
		model.Resolution = &bbmodel.Resolution{Width: width, Height: height}
		model.Textures = append(model.Textures, bbmodel.Texture{Name: "palette", ID: "0", Width: width, Height: height, UVWidth: width, UVHeight: height, RenderMode: "default", RenderSides: "auto", Visible: true, Internal: true, Saved: true, UUID: bbmodel.NewUUID(), Source: src})
	} else {
		model.Resolution = &bbmodel.Resolution{Width: 16, Height: 16}
	}

	var emit func(*partWork) bbmodel.OutlinerNode
	emit = func(w *partWork) bbmodel.OutlinerNode {
		node := bbmodel.OutlinerNode{UUID: w.group.UUID, IsOpen: true, Children: []any{}}
		texID := 0
		for i, box := range w.boxes {
			from, to, center, rotation, _ := mapTransformedBox(box, w.matrix, w.bone.AWOrigin)
			origin := w.group.Origin
			if rotation != [3]float32{} {
				origin = center
			}
			elem := bbmodel.Element{Name: fmt.Sprintf("%s_%d", w.group.Name, i), CostumeSlot: w.group.CostumeSlot, BoxUV: false, RenderOrder: "default", From: from, To: to, Origin: origin, Rotation: rotation, UUID: bbmodel.NewUUID(), Type: "cube", Faces: make(map[string]bbmodel.Face, 6)}
			if box.CubeType.IsGlowing() {
				v := 15
				elem.LightEmission = &v
			}
			if box.CubeType.IsGlass() {
				v := false
				elem.Shade = &v
			}
			for awFace := 0; awFace < 6; awFace++ {
				if idx := w.regions[i][awFace]; idx >= 0 {
					elem.Faces[AWFaceToBBName[awFace]] = bbmodel.Face{UV: atlas.GetUV(idx), Texture: &texID}
				} else {
					elem.Faces[AWFaceToBBName[awFace]] = bbmodel.Face{}
				}
			}
			model.Elements = append(model.Elements, elem)
			node.Children = append(node.Children, elem.UUID)
			w.group.Children = append(w.group.Children, elem.UUID)
		}
		for _, child := range w.children {
			node.Children = append(node.Children, emit(child))
			w.group.Children = append(w.group.Children, child.group.UUID)
		}
		model.Groups = append(model.Groups, w.group)
		return node
	}
	for _, target := range targetRootOrder {
		root := newTargetGroup(target)
		node := bbmodel.OutlinerNode{UUID: root.UUID, IsOpen: true, Children: []any{}}
		for _, w := range works {
			if w.target != target {
				continue
			}
			node.Children = append(node.Children, emit(w))
			root.Children = append(root.Children, w.group.UUID)
		}
		if len(node.Children) > 0 {
			model.Groups = append(model.Groups, root)
			model.Outliner = append(model.Outliner, node)
		}
	}
	appendWingAnimations(model, wings)
	return model, nil
}
