package timg

import (
	"fmt"

	"github.com/haryoiro/yutemal/pkg/timg/detect"
	"github.com/haryoiro/yutemal/pkg/timg/protocol"
)

// TerminalImage マルチプロトコル対応の端末画像表示
type TerminalImage struct {
	proto protocol.Protocol
}

// New 自動検出されたプロトコルでインスタンスを作成
func New() *TerminalImage {
	return &TerminalImage{proto: detect.Auto()}
}

// NewWithProtocol 指定プロトコルでインスタンスを作成
func NewWithProtocol(protoType protocol.Type) *TerminalImage {
	return &TerminalImage{proto: protocol.New(protoType)}
}

// Protocol プロトコルタイプを返す
func (ti *TerminalImage) Protocol() protocol.Type {
	if err := ti.validate(); err != nil {
		return protocol.TypeNone
	}
	return ti.proto.Type()
}

// ProtocolName プロトコル名を返す
func (ti *TerminalImage) ProtocolName() string {
	if err := ti.validate(); err != nil {
		return "None"
	}
	return ti.proto.Name()
}

// IsSupported グラフィックスプロトコルがサポートされているか
func (ti *TerminalImage) IsSupported() bool {
	return ti.validate() == nil && ti.proto.Type() != protocol.TypeNone
}

// validate プロトコルの存在を検証
func (ti *TerminalImage) validate() error {
	if ti.proto == nil {
		return fmt.Errorf("サポートされたグラフィックスプロトコルが検出されませんでした")
	}
	return nil
}

// Display 画像を表示
func (ti *TerminalImage) Display(imagePath string, opts ...protocol.DisplayOption) error {
	if err := ti.validate(); err != nil {
		return err
	}
	return ti.proto.Display(imagePath, opts...)
}

// Clear 画像をクリア
func (ti *TerminalImage) Clear() {
	if err := ti.validate(); err != nil {
		return
	}
	ti.proto.Clear()
}

// ClearArea 指定領域をクリア
func (ti *TerminalImage) ClearArea(pos Position) {
	if err := ti.validate(); err != nil {
		return
	}
	ti.proto.ClearArea(pos)
}

// getKittyProtocol Kittyプロトコルを取得
func (ti *TerminalImage) getKittyProtocol() (protocol.KittyProtocol, error) {
	if err := ti.validate(); err != nil {
		return nil, err
	}
	if kp, ok := ti.proto.(protocol.KittyProtocol); ok {
		return kp, nil
	}
	return nil, fmt.Errorf("この機能はKittyプロトコルでのみサポートされています")
}

// ClearByID IDでクリア（Kittyのみ）
func (ti *TerminalImage) ClearByID(id uint32) error {
	kp, err := ti.getKittyProtocol()
	if err != nil {
		return err
	}
	return kp.ClearByID(id)
}
