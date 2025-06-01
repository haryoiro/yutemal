package protocol

import (
	"encoding/base64"
	"fmt"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// iterm2 iTerm2 Inline Imagesプロトコルの実装
type iterm2 struct{}

func newITerm2() Protocol {
	return &iterm2{}
}

func (i *iterm2) Type() Type {
	return TypeITerm2
}

func (i *iterm2) Name() string {
	return "iTerm2 Inline Images"
}

func (i *iterm2) Display(imagePath string, opts ...DisplayOption) error {
	options := ApplyOptions(opts)

	data, err := internal.ReadImageFile(imagePath)
	if err != nil {
		return err
	}

	// 位置指定がある場合はカーソル移動
	if options.X > 0 || options.Y > 0 {
		internal.MoveCursor(options.X, options.Y)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	// パラメータ構築
	params := "inline=1"

	// セルサイズ指定
	if options.Width > 0 && options.Height > 0 {
		params += fmt.Sprintf(";width=%d;height=%d", options.Width, options.Height)
	}

	// ピクセルサイズ指定（iTerm2ではpxを付ける）
	if options.PixelWidth > 0 && options.PixelHeight > 0 {
		params += fmt.Sprintf(";width=%dpx;height=%dpx", options.PixelWidth, options.PixelHeight)
	}

	fmt.Printf("\x1b]1337;File=%s:%s\a", params, encoded)
	return nil
}

func (i *iterm2) Clear() {
	// iTerm2には特定のクリアコマンドがない
}

func (i *iterm2) ClearArea(pos Position) {
	internal.ClearAreaWithDimensions(pos.X, pos.Y, pos.Width, pos.Height)
}
