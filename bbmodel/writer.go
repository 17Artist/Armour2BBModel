// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package bbmodel

import (
	"encoding/json"
	"io"
)

// WriteJSON 将 Model 序列化为 JSON 写入 writer
func WriteJSON(w io.Writer, m *Model) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(m)
}

// ToJSON 将 Model 序列化为 JSON 字节
func ToJSON(m *Model) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
