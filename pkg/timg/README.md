# timg

端末で画像を表示するためのGoライブラリ。複数のグラフィックプロトコルに対応し、自動検出機能を提供。

## 使い方

```go
package main

import (
    "log"
    "github.com/haryoiro/yutemal/pkg/timg"
)

func main() {
    // 自動検出
    ti := timg.New()

    // 画像を表示
    err := ti.Display("image.png")
    if err != nil {
        log.Fatal(err)
    }

    // クリア
    ti.Clear()
}
```

## API

### 基本的な使い方

```go
// インスタンス作成
ti := timg.New()                              // 自動検出
ti := timg.NewWithProtocol(timg.ProtocolKitty) // プロトコル指定

// 画像表示
ti.Display("image.png")                        // 基本表示
ti.Display("image.png", timg.WithPosition(10, 5))  // 位置指定
ti.Display("image.png", timg.WithSize(20, 10))     // サイズ指定
ti.Clear()                                     // クリア
ti.ClearArea(pos)                              // 領域クリア

// 情報取得
ti.Protocol()                                  // プロトコルタイプ
ti.ProtocolName()                              // プロトコル名
ti.IsSupported()                               // サポート確認
```

### 表示オプション

```go
// 位置指定
ti.Display("image.png", timg.WithPosition(10, 5))

// サイズ指定（セル単位）
ti.Display("image.png", timg.WithSize(20, 10))

// ピクセルサイズ指定
ti.Display("image.png", timg.WithPixelSize(800, 600))

// クロップ
ti.Display("image.png", timg.WithCrop(100, 50, 200, 150))

// 複数のオプションを組み合わせ
ti.Display("image.png",
    timg.WithPosition(10, 5),
    timg.WithSize(20, 10),
    timg.WithCrop(100, 50, 200, 150),
)

// 領域クリア用の位置指定
pos := timg.Position{
    X: 10, Y: 5,
    Width: 20, Height: 10,
}
ti.ClearArea(pos)
```

### Kitty拡張機能

```go
// チャンク転送
ti.Display("large.png", timg.WithChunkSize(4096))

// ID管理
ti.Display("img1.png", timg.WithID(1))
ti.ClearByID(1)

// 組み合わせ使用
ti.Display("large.png",
    timg.WithID(1),
    timg.WithChunkSize(4096),
    timg.WithSize(100, 50),
)
```

## 対応プロトコル

- **Kitty Graphics**: Kitty、WezTerm、Foot、Ghostty
- **iTerm2 Inline Images**: iTerm2
- **Sixel**: xterm、mlterm、WezTerm、MinTTY等
- **Terminal Graphics (w3m-img)**: 多くのLinux端末

## ユーティリティ

```go
// 端末サイズ取得
cols, rows, err := timg.GetTerminalSize()

// Sixel検出
sixelCaps := timg.DetectSixelCapabilities()
```
