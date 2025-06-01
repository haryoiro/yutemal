package protocol

type Type int

const (
	TypeNone Type = iota
	TypeKitty
	TypeITerm2
	TypeSixel
	TypeTerminalGraphics
)

// Position 画像表示位置とサイズ
type Position struct {
	X      int // カラム位置 (1から)
	Y      int // 行位置 (1から)
	Width  int // セル幅
	Height int // セル高さ

	// クロップパラメータ (ピクセル単位、オプション)
	CropX      int
	CropY      int
	CropWidth  int // 0=元のまま
	CropHeight int // 0=元のまま
}

// Protocol 端末グラフィックプロトコルのベースインターフェース
type Protocol interface {
	Type() Type
	Name() string
	Display(imagePath string, opts ...DisplayOption) error
	Clear()
	ClearArea(pos Position)
}

// KittyProtocol Kitty固有機能を持つプロトコル
type KittyProtocol interface {
	Protocol
	ClearByID(id uint32) error
}

func New(protoType Type) Protocol {
	switch protoType {
	case TypeKitty:
		return newKitty()
	case TypeITerm2:
		return newITerm2()
	case TypeSixel:
		return newSixel()
	case TypeTerminalGraphics:
		return newTerminalGraphics()
	default:
		return nil
	}
}
