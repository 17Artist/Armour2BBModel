// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package skin

import (
	"bufio"
	"fmt"
	"io"
)

const headerMagic int32 = 0x534B494E // "SKIN"

// Read 从输入流读取 AW skin 文件，自动检测版本
func Read(r io.Reader) (*SkinFile, error) {
	br := bufio.NewReader(r)

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

	if fileVersion >= 20 {
		return readV20(br, fileVersion)
	} else if fileVersion == 13 {
		return readV13(br, fileVersion)
	} else if fileVersion >= 1 {
		return readV12(br, fileVersion)
	}

	return nil, fmt.Errorf("unsupported file version: %d", fileVersion)
}
