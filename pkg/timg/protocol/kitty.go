package protocol

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// kitty Kitty Graphics Protocolの実装
type kitty struct{}

func newKitty() KittyProtocol {
	return &kitty{}
}

func (k *kitty) Type() Type {
	return TypeKitty
}

func (k *kitty) Name() string {
	return "Kitty Graphics Protocol"
}

func (k *kitty) Display(imagePath string, opts ...DisplayOption) error {
	options := ApplyOptions(opts)

	// チャンク転送モードの場合
	if options.ChunkSize > 0 {
		return k.displayChunked(imagePath, options)
	}

	// 通常の転送
	data, err := internal.ReadImageFile(imagePath)
	if err != nil {
		return err
	}

	// 位置指定がある場合はカーソル移動
	if options.X > 0 || options.Y > 0 {
		internal.MoveCursor(options.X, options.Y)
	}

	// コマンド構築
	cmd := "f=100,a=T"

	// 位置指定がある場合
	if options.X > 0 || options.Y > 0 {
		cmd += ",p=1" // カーソル位置からの相対位置
	}

	// サイズ指定
	if options.Width > 0 && options.Height > 0 {
		cmd += fmt.Sprintf(",c=%d,r=%d", options.Width, options.Height)
	}

	// クロップ指定
	if options.CropWidth > 0 && options.CropHeight > 0 {
		cmd += fmt.Sprintf(",x=%d,y=%d,w=%d,h=%d",
			options.CropX, options.CropY, options.CropWidth, options.CropHeight)
	}

	// ピクセルサイズ指定
	if options.PixelWidth > 0 && options.PixelHeight > 0 {
		cmd += fmt.Sprintf(",s=%d,v=%d", options.PixelWidth, options.PixelHeight)
	}

	// ID指定
	if options.ID > 0 {
		cmd += fmt.Sprintf(",i=%d", options.ID)
	}

	fmt.Printf("\x1b_G%s;%s\x1b\\", cmd, base64.StdEncoding.EncodeToString(data))
	return nil
}

func (k *kitty) Clear() {
	fmt.Print("\x1b_Ga=d\x1b\\")
}

func (k *kitty) ClearArea(pos Position) {
	internal.ClearAreaWithDimensions(pos.X, pos.Y, pos.Width, pos.Height)
}

// displayChunked チャンク転送モードでの表示（内部使用）
func (k *kitty) displayChunked(imagePath string, options *DisplayOptions) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	chunkSize := options.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 4096 // デフォルトチャンクサイズ
	}

	buffer := make([]byte, chunkSize)
	isFirst := true

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read chunk: %w", err)
		}

		if n == 0 {
			break
		}

		encoded := base64.StdEncoding.EncodeToString(buffer[:n])

		if isFirst {
			// 最初のチャンクにはすべてのパラメータを含める
			cmd := "f=100,a=T,m=1"

			if options.Width > 0 && options.Height > 0 {
				cmd += fmt.Sprintf(",c=%d,r=%d", options.Width, options.Height)
			}

			if options.ID > 0 {
				cmd += fmt.Sprintf(",i=%d", options.ID)
			}

			fmt.Printf("\x1b_G%s;%s\x1b\\", cmd, encoded)
			isFirst = false
		} else {
			fmt.Printf("\x1b_Gm=1;%s\x1b\\", encoded)
		}
	}

	fmt.Print("\x1b_Gm=0;\x1b\\")

	return nil
}

func (k *kitty) ClearByID(id uint32) error {
	fmt.Printf("\x1b_Ga=d,d=i,i=%d\x1b\\", id)
	return nil
}
