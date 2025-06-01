package timg

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/haryoiro/yutemal/pkg/timg/detect"
)

func GetTerminalSize() (cols, rows int, err error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	_, err = fmt.Sscanf(string(output), "%d %d", &rows, &cols)
	return cols, rows, err
}

// 互換性のために検出関数を再エクスポート
var (
	DetectCapabilities           = detect.Capabilities
	DetectKittyProtocolWithQuery = detect.KittyProtocolWithQuery
	DetectSixelCapabilities      = detect.SixelCapabilities
)

