package protocol

import (
	"fmt"
	"os/exec"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// sixel Sixelプロトコルの実装
type sixel struct{}

func newSixel() Protocol {
	return &sixel{}
}

func (s *sixel) Type() Type {
	return TypeSixel
}

func (s *sixel) Name() string {
	return "Sixel"
}

func (s *sixel) Display(imagePath string, opts ...DisplayOption) error {
	options := ApplyOptions(opts)

	encoder, baseArgs := getSixelEncoderCommand()
	if encoder == "" {
		return fmt.Errorf("no sixel encoder found (install ImageMagick or img2sixel)")
	}

	// 位置指定がある場合はカーソル移動
	if options.X > 0 || options.Y > 0 {
		internal.MoveCursor(options.X, options.Y)
	}

	// 表示サイズの計算
	var displayWidth, displayHeight int

	// ピクセルサイズが指定されている場合
	if options.PixelWidth > 0 && options.PixelHeight > 0 {
		displayWidth = options.PixelWidth
		displayHeight = options.PixelHeight
	} else if options.Width > 0 && options.Height > 0 {
		// セルサイズからピクセルサイズを計算 (8x16セルサイズを想定)
		displayWidth = options.Width * 8
		displayHeight = options.Height * 16
	}

	// エンコーダー固有のオプション構築
	var args []string
	switch encoder {
	case "convert":
		args = []string{imagePath}

		// クロップ指定
		if options.CropWidth > 0 && options.CropHeight > 0 {
			args = append(args, "-crop", fmt.Sprintf("%dx%d+%d+%d",
				options.CropWidth, options.CropHeight, options.CropX, options.CropY))
		}

		// サイズ指定がある場合
		if displayWidth > 0 && displayHeight > 0 {
			args = append(args, "-geometry", fmt.Sprintf("%dx%d!", displayWidth, displayHeight))
		} else {
			// デフォルトサイズ
			args = append(args, "-geometry", "800x600>")
		}
		args = append(args, "sixel:-")

	case "img2sixel":
		args = []string{}

		// クロップ指定
		if options.CropWidth > 0 && options.CropHeight > 0 {
			args = append(args,
				"--crop-offset", fmt.Sprintf("%d,%d", options.CropX, options.CropY),
				"--crop-size", fmt.Sprintf("%dx%d", options.CropWidth, options.CropHeight))
		}

		// サイズ指定がある場合
		if displayWidth > 0 && displayHeight > 0 {
			args = append(args, "-w", fmt.Sprintf("%d", displayWidth),
				"-h", fmt.Sprintf("%d", displayHeight))
		} else {
			// デフォルトサイズ
			args = append(args, "-w", "800", "-h", "600")
		}
		args = append(args, imagePath)

	default:
		// その他のエンコーダーはベース引数とファイル名のみ
		args = append(baseArgs, imagePath)
	}

	cmd := exec.Command(encoder, args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to convert to sixel: %w", err)
	}

	fmt.Print(string(output))
	return nil
}

func (s *sixel) Clear() {
	fmt.Print("\x1b[2J\x1b[H")
}

func (s *sixel) ClearArea(pos Position) {
	internal.ClearAreaWithDimensions(pos.X, pos.Y, pos.Width, pos.Height)
}

// getSixelEncoderCommand Sixel形式への変換コマンドを返す
func getSixelEncoderCommand() (string, []string) {
	encoders := []struct {
		cmd  string
		args []string
	}{
		{"convert", []string{"-geometry", "800x600>", "sixel:-"}},
		{"img2sixel", []string{"-w", "800", "-h", "600"}},
		{"sixel", []string{}},
	}

	for _, encoder := range encoders {
		if _, err := exec.LookPath(encoder.cmd); err == nil {
			return encoder.cmd, encoder.args
		}
	}

	return "", nil
}
