package protocol

// DisplayOptions 画像表示用のオプション
type DisplayOptions struct {
	// 表示位置（1から始まる）
	X int
	Y int

	// 表示サイズ（セル単位）
	Width  int
	Height int

	// クロップ（ピクセル単位、オプション）
	CropX      int
	CropY      int
	CropWidth  int
	CropHeight int

	// Kitty固有
	ID        uint32 // 画像ID（0の場合は自動）
	ChunkSize int    // チャンクサイズ（0の場合はデフォルト）

	// サイズ指定（ピクセル単位、0の場合は元のサイズ）
	PixelWidth  int
	PixelHeight int
}

// DisplayOption オプション設定関数
type DisplayOption func(*DisplayOptions)

// WithPosition 表示位置を設定
func WithPosition(x, y int) DisplayOption {
	return func(o *DisplayOptions) {
		o.X = x
		o.Y = y
	}
}

// WithSize セルサイズを設定
func WithSize(width, height int) DisplayOption {
	return func(o *DisplayOptions) {
		o.Width = width
		o.Height = height
	}
}

// WithCrop クロップ領域を設定
func WithCrop(x, y, width, height int) DisplayOption {
	return func(o *DisplayOptions) {
		o.CropX = x
		o.CropY = y
		o.CropWidth = width
		o.CropHeight = height
	}
}

// WithID Kitty画像IDを設定
func WithID(id uint32) DisplayOption {
	return func(o *DisplayOptions) {
		o.ID = id
	}
}

// WithChunkSize チャンクサイズを設定
func WithChunkSize(size int) DisplayOption {
	return func(o *DisplayOptions) {
		o.ChunkSize = size
	}
}

// WithPixelSize ピクセルサイズを設定
func WithPixelSize(width, height int) DisplayOption {
	return func(o *DisplayOptions) {
		o.PixelWidth = width
		o.PixelHeight = height
	}
}

// ApplyOptions オプションを適用
func ApplyOptions(opts []DisplayOption) *DisplayOptions {
	options := &DisplayOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// ToPosition DisplayOptionsをPositionに変換
func (o *DisplayOptions) ToPosition() Position {
	return Position{
		X:          o.X,
		Y:          o.Y,
		Width:      o.Width,
		Height:     o.Height,
		CropX:      o.CropX,
		CropY:      o.CropY,
		CropWidth:  o.CropWidth,
		CropHeight: o.CropHeight,
	}
}
