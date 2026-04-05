// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"syscall/js"

	"github.com/17Artist/Armour2BBModel/converter"
	"github.com/17Artist/Armour2BBModel/skin"
)

func convert(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "no data"})
	}

	jsData := args[0]
	length := jsData.Get("length").Int()
	data := make([]byte, length)
	js.CopyBytesToGo(data, jsData)

	sf, err := skin.Read(bytes.NewReader(data))
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": "解析失败: " + err.Error()})
	}

	model, err := converter.Convert(sf)
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

	return js.ValueOf(map[string]interface{}{
		"data":     result,
		"elements": len(model.Elements),
		"skinType": sf.SkinType,
	})
}

func main() {
	js.Global().Set("convertArmour", js.FuncOf(convert))
	js.Global().Call("eval", "if(window._wasmReady) window._wasmReady()")
	select {}
}
