// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

// Command convert provides the same conversion pipeline without a browser.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/17Artist/Armour2BBModel/converter"
	"github.com/17Artist/Armour2BBModel/skin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	input := flag.String("input", "", "input .armour/.awsk file")
	output := flag.String("output", "", "output .bbmodel file (default: beside input)")
	keepFaces := flag.Bool("keep-coincident-faces", false, "keep coincident attachment surfaces for independent editing")
	slot := flag.String("slot", "", "export one ArcartX slot: HEAD, BODY, LEGS, FEET or DECORATION")
	flag.Parse()
	if *input == "" {
		return fmt.Errorf("usage: convert -input skin.armour [-output model.bbmodel]")
	}
	if *output == "" {
		*output = strings.TrimSuffix(*input, filepath.Ext(*input)) + ".bbmodel"
	}
	inPath, err := filepath.Abs(*input)
	if err != nil {
		return err
	}
	outPath, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if strings.EqualFold(inPath, outPath) {
		return fmt.Errorf("input and output paths must differ")
	}
	f, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer f.Close()
	sf, err := skin.Read(f)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	model, err := converter.ConvertWithOptions(sf, converter.Options{KeepCoincidentFaces: *keepFaces})
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}
	if *slot != "" {
		model, err = model.ForCostumeSlot(strings.ToUpper(*slot))
		if err != nil {
			return err
		}
	}
	data, err := json.Marshal(model)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("%s: %d cuboids, %d groups, %d textures\n", outPath, len(model.Elements), len(model.Groups), len(model.Textures))
	return nil
}
