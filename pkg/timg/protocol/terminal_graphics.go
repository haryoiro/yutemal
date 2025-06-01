package protocol

import (
	"fmt"
	"os/exec"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// terminalGraphics Terminal Graphics (w3m-img)プロトコルの実装
type terminalGraphics struct{}

func newTerminalGraphics() Protocol {
	return &terminalGraphics{}
}

func (t *terminalGraphics) Type() Type {
	return TypeTerminalGraphics
}

func (t *terminalGraphics) Name() string {
	return "Terminal Graphics (w3m-img)"
}

func (t *terminalGraphics) Display(imagePath string, opts ...DisplayOption) error {
	options := ApplyOptions(opts)

	// 位置指定がある場合はカーソル移動
	if options.X > 0 || options.Y > 0 {
		internal.MoveCursor(options.X, options.Y)
	}

	// w3m-imgは限定的なオプションサポート
	// 基本的にはファイルを表示するだけ
	cmd := exec.Command("w3m-img", imagePath)

	// サイズ指定がある場合の警告
	if options.Width > 0 || options.Height > 0 || options.PixelWidth > 0 || options.PixelHeight > 0 {
		// w3m-imgはサイズ指定をサポートしていないが、エラーにはしない
	}

	return cmd.Run()
}

func (t *terminalGraphics) Clear() {
	fmt.Print("\x1b[2J\x1b[H")
}

func (t *terminalGraphics) ClearArea(pos Position) {
	internal.ClearAreaWithDimensions(pos.X, pos.Y, pos.Width, pos.Height)
}
