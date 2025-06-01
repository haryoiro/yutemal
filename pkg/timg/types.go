package timg

import "github.com/haryoiro/yutemal/pkg/timg/protocol"

// Position protocol.Positionのエイリアス
type Position = protocol.Position

type Protocol = protocol.Type

const (
	ProtocolNone             = protocol.TypeNone
	ProtocolKitty            = protocol.TypeKitty
	ProtocolITerm2           = protocol.TypeITerm2
	ProtocolSixel            = protocol.TypeSixel
	ProtocolTerminalGraphics = protocol.TypeTerminalGraphics
)

// DisplayOption 表示オプション関数のエイリアス
type DisplayOption = protocol.DisplayOption

// 表示オプション関数のエクスポート
var (
	WithPosition  = protocol.WithPosition
	WithSize      = protocol.WithSize
	WithCrop      = protocol.WithCrop
	WithID        = protocol.WithID
	WithChunkSize = protocol.WithChunkSize
	WithPixelSize = protocol.WithPixelSize
)
