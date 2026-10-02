// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"fmt"
	"strconv"
	"strings"
)

// bindPartProperties matches Skin.Builder.updatePropertiesIfNeeded: outfit indices
// are cumulative root-part ends, not part IDs, type IDs, or a list of per-part indices.
// SkinProperties.Stub resolves parameter "key" as "key"+equipmentIndex. Per-part
// PPTS settings on children remain independent, as in the official SkinPart builder.
func bindPartProperties(sf *SkinFile) error {
	stack := append([]*PartData(nil), sf.Parts...)
	seen := map[*PartData]bool{}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p == nil || seen[p] {
			return fmt.Errorf("invalid source part hierarchy while binding properties")
		}
		seen[p] = true
		if len(seen) > maxParts {
			return fmt.Errorf("source part limit exceeded while binding properties")
		}
		if p.Properties == nil {
			p.Properties = map[string]string{}
		}
		p.EquipmentIndex = nil
		stack = append(stack, p.Children...)
	}
	if sf.SkinType == "armourers:wings" {
		props := sf.Properties
		if props == nil {
			props = map[string]string{}
		}
		for _, p := range sf.Parts {
			p.Properties = props
		}
	}
	indices := sf.Properties["partIndexs"]
	if indices == "" {
		return nil
	}
	// Java String.split drops trailing empty entries, including trailing colons.
	trimmed := strings.TrimRight(indices, ":")
	if trimmed == "" {
		return nil
	}
	entries := strings.Split(trimmed, ":")
	if len(entries) > maxParts {
		return fmt.Errorf("partIndexs equipment limit exceeded")
	}
	start := 0
	for equipment, text := range entries {
		end64, err := strconv.ParseInt(text, 10, 32)
		if err != nil || end64 < 0 {
			return fmt.Errorf("invalid partIndexs endpoint %q at equipment %d", text, equipment)
		}
		end := int(end64)
		if end < start {
			return fmt.Errorf("partIndexs endpoints decrease at equipment %d", equipment)
		}
		props := equipmentProperties(sf.Properties, equipment, indices)
		// The official reader ignores endpoints beyond the available parts. Clamp the
		// loop to avoid work proportional to an attacker-controlled 32-bit endpoint.
		for i := start; i < end && i < len(sf.Parts); i++ {
			index := equipment
			sf.Parts[i].EquipmentIndex = &index
			sf.Parts[i].Properties = props
		}
		start = end
	}
	return nil
}

func equipmentProperties(source map[string]string, index int, indices string) map[string]string {
	// Stub.isEmpty checks the original property container, not only this index's
	// values. Keep the actual outfit metadata so a numbered view without explicit
	// wing parameters still uses official defaults instead of being mistaken for
	// a genuinely empty per-part PPTS container.
	props := map[string]string{"partIndexs": indices}
	for key, value := range source {
		split := len(key)
		for split > 0 && key[split-1] >= '0' && key[split-1] <= '9' {
			split--
		}
		if split == len(key) || split == 0 {
			continue
		}
		// Match the entire decimal suffix; equipment 4 must not read key14. Official
		// resolveKey uses canonical decimal indices, so key04 is not index 4 either.
		if key[split:] == strconv.Itoa(index) {
			props[key[:split]] = value
		}
	}
	return props
}
