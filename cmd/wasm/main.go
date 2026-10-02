// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/17Artist/Armour2BBModel/converter"
	"github.com/17Artist/Armour2BBModel/skin"
)

func convert(this js.Value, args []js.Value) (response interface{}) {
	defer func() {
		if failure := recover(); failure != nil {
			response = js.ValueOf(map[string]interface{}{"error": fmt.Sprintf("转换失败: %v", failure)})
		}
	}()
	started := time.Now()
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "no data"})
	}

	jsData := args[0]
	if !jsData.InstanceOf(js.Global().Get("Uint8Array")) {
		return js.ValueOf(map[string]interface{}{"error": "请提供 Uint8Array 格式的文件数据"})
	}
	length := jsData.Get("length").Int()
	if length == 0 || length > 64*1024*1024 {
		return js.ValueOf(map[string]interface{}{"error": "文件不能为空，且不能超过 64 MB"})
	}
	data := make([]byte, length)
	js.CopyBytesToGo(data, jsData)

	sf, err := skin.Read(bytes.NewReader(data))
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": "解析失败: " + err.Error()})
	}

	options := converter.Options{}
	if len(args) > 1 && args[1].Type() == js.TypeObject && !args[1].IsNull() {
		value := args[1].Get("keepCoincidentFaces")
		if value.Type() == js.TypeBoolean {
			options.KeepCoincidentFaces = value.Bool()
		}
	}
	model, err := converter.ConvertWithOptions(sf, options)
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": "转换失败: " + err.Error()})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(model); err != nil {
		return js.ValueOf(map[string]interface{}{"error": "JSON编码失败: " + err.Error()})
	}

	result := js.Global().Get("Uint8Array").New(buf.Len())
	js.CopyBytesToJS(result, buf.Bytes())

	inputVoxels := 0
	var countParts func([]*skin.PartData)
	countParts = func(parts []*skin.PartData) {
		for _, part := range parts {
			inputVoxels += len(part.Cubes)
			countParts(part.Children)
		}
	}
	countParts(sf.Parts)
	ratio := 0.0
	if inputVoxels > 0 {
		ratio = 1 - float64(len(model.Elements))/float64(inputVoxels)
	}
	outputParts := make([]interface{}, 0, len(model.Groups))
	elementIDs := make(map[string]bool, len(model.Elements))
	for _, element := range model.Elements {
		elementIDs[element.UUID] = true
	}
	for _, group := range model.Groups {
		count := 0
		for _, child := range group.Children {
			if uuid, ok := child.(string); ok && elementIDs[uuid] {
				count++
			}
		}
		bone := group.Attachment
		if bone == "" {
			bone = group.Name
		}
		outputParts = append(outputParts, map[string]interface{}{"bone": bone, "groupUUID": group.UUID, "sourcePart": group.SourcePart, "sourceName": group.SourceName, "outputCuboids": count})
	}
	author := sf.Properties["authorName"]
	if author == "" {
		author = sf.Properties["author"]
	}
	warnings := make([]interface{}, len(sf.Warnings))
	for i, warning := range sf.Warnings {
		warnings[i] = warning
	}

	return js.ValueOf(map[string]interface{}{
		"data":     result,
		"elements": len(model.Elements),
		"skinType": sf.SkinType,
		"warnings": warnings,
		"stats": map[string]interface{}{
			"inputVoxels":            inputVoxels,
			"outputCuboids":          len(model.Elements),
			"resolvedCoplanarTexels": model.SurfaceDeduplicatedTexels,
			"mergeRatio":             ratio,
			"elapsedMs":              float64(time.Since(started).Microseconds()) / 1000,
			"atlasWidth":             model.Resolution.Width,
			"atlasHeight":            model.Resolution.Height,
			"parts":                  outputParts,
		},
		"metadata": map[string]interface{}{"version": sf.FileVersion, "author": author, "name": model.Name},
	})
}

func main() {
	js.Global().Set("convertArmour", js.FuncOf(convert))
	if ready := js.Global().Get("_wasmReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
