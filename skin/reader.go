// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const headerMagic int32 = 0x534B494E // "SKIN"

// Read 从输入流读取 AW skin 文件，自动检测版本
func Read(r io.Reader) (*SkinFile, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading skin file: %w", err)
	}
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("skin file exceeds %d MiB", MaxInputBytes>>20)
	}
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("opening compressed skin: %w", err)
		}
		data, err = io.ReadAll(io.LimitReader(gr, maxDecodedBytes+1))
		gr.Close()
		if err != nil {
			return nil, fmt.Errorf("decompressing skin: %w", err)
		}
		if len(data) > maxDecodedBytes {
			return nil, fmt.Errorf("decompressed skin exceeds limit")
		}
	}
	br := bytes.NewReader(data)

	firstInt, err := readInt32(br)
	if err != nil {
		return nil, fmt.Errorf("reading file version: %w", err)
	}

	var fileVersion int
	if firstInt == headerMagic {
		// v20+: magic header 后跟版本号
		v, err := readInt32(br)
		if err != nil {
			return nil, fmt.Errorf("reading v20+ version: %w", err)
		}
		fileVersion = int(v)
	} else {
		fileVersion = int(firstInt)
	}

	var sf *SkinFile
	if fileVersion >= 20 && fileVersion <= 25 {
		sf, err = readV20(br, fileVersion)
	} else if fileVersion == 13 {
		sf, err = readV13(br, fileVersion)
	} else if fileVersion >= 1 && fileVersion <= 12 {
		sf, err = readV12(br, fileVersion)
	} else {
		return nil, fmt.Errorf("unsupported file version: %d", fileVersion)
	}
	if err != nil {
		return nil, err
	}
	if br.Len() != 0 {
		return nil, fmt.Errorf("unexpected %d trailing bytes", br.Len())
	}
	if err = bindPartProperties(sf); err != nil {
		return nil, err
	}
	if len(sf.PaintData) > 0 {
		sf.Warnings = append(sf.Warnings, "玩家彩绘数据未导出，仅转换体素几何")
	}
	return sf, nil
}
