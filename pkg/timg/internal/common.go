package internal

import (
	"fmt"
	"os"
	"strings"
)

func ReadImageFile(imagePath string) ([]byte, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	return data, nil
}

func ClearAreaWithDimensions(x, y, width, height int) {
	// カーソル位置を保存
	fmt.Print("\x1b[s")

	// 指定された矩形領域をクリア
	for row := 0; row < height; row++ {
		fmt.Printf("\x1b[%d;%dH", y+row, x)
		for col := 0; col < width; col++ {
			fmt.Print(" ")
		}
	}

	// カーソル位置を復元
	fmt.Print("\x1b[u")
}

func MoveCursor(x, y int) {
	fmt.Printf("\x1b[%d;%dH", y, x)
}

func GetTermEnv() (term, termProgram string) {
	term = os.Getenv("TERM")
	termProgram = os.Getenv("TERM_PROGRAM")
	return term, termProgram
}

func IsKittyTerminal(term, termProgram string) bool {
	return strings.Contains(term, "kitty") ||
		strings.Contains(termProgram, "kitty") ||
		os.Getenv("KITTY_WINDOW_ID") != ""
}

func IsInteractiveTerminal() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}
