package bbmodel

import "fmt"

// ForCostumeSlot returns an independently exportable ArcartX slot with its
// original canonical roots, coordinates, UUIDs and atlas. It never copies an
// element into a second slot and does not mutate the complete model.
func (m *Model) ForCostumeSlot(slot string) (*Model, error) {
	switch slot {
	case "HEAD", "BODY", "LEGS", "FEET", "DECORATION":
	default:
		return nil, fmt.Errorf("unknown costume slot %q", slot)
	}
	if m == nil {
		return nil, fmt.Errorf("nil model")
	}
	result := *m
	result.Name = m.Name + "-" + slot
	result.Elements, result.Groups, result.Outliner, result.Animations = []Element{}, []Group{}, []any{}, nil
	// This aggregate counter describes the complete conversion, not one slice.
	result.SurfaceDeduplicatedTexels = 0
	elements := map[string]bool{}
	for _, element := range m.Elements {
		if element.CostumeSlot == slot {
			result.Elements = append(result.Elements, element)
			elements[element.UUID] = true
		}
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("costume slot %s is empty", slot)
	}
	groups := map[string]bool{}
	var trim func(any) (any, error)
	trim = func(value any) (any, error) {
		if id, ok := value.(string); ok {
			if elements[id] {
				return id, nil
			}
			return nil, nil
		}
		node, ok := value.(OutlinerNode)
		if !ok {
			return nil, fmt.Errorf("unsupported outliner node %T", value)
		}
		children := []any{}
		for _, child := range node.Children {
			kept, err := trim(child)
			if err != nil {
				return nil, err
			}
			if kept != nil {
				children = append(children, kept)
			}
		}
		if len(children) == 0 {
			return nil, nil
		}
		node.Children = children
		groups[node.UUID] = true
		return node, nil
	}
	for _, root := range m.Outliner {
		kept, err := trim(root)
		if err != nil {
			return nil, err
		}
		if kept != nil {
			result.Outliner = append(result.Outliner, kept)
		}
	}
	for _, group := range m.Groups {
		if !groups[group.UUID] {
			continue
		}
		originalChildren := group.Children
		group.Children = []any{}
		for _, child := range originalChildren {
			if id, ok := child.(string); ok && (elements[id] || groups[id]) {
				group.Children = append(group.Children, id)
			}
		}
		result.Groups = append(result.Groups, group)
	}
	for _, animation := range m.Animations {
		animators := map[string]Animator{}
		for id, animator := range animation.Animators {
			if groups[id] {
				animators[id] = animator
			}
		}
		if len(animators) > 0 {
			animation.Animators = animators
			result.Animations = append(result.Animations, animation)
		}
	}
	return &result, nil
}
