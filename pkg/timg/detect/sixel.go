package detect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// SixelCapabilitiesInfo Sixelサポートの詳細情報
type SixelCapabilitiesInfo struct {
	Supported       bool
	DetectionMethod string
	TerminalType    string
	InTmux          bool
	TmuxSixelReady  bool
	EncoderCommand  string   // 利用可能なエンコーダコマンド
	EncoderArgs     []string // エンコーダコマンドの引数
}

func SixelCapabilities() SixelCapabilitiesInfo {
	term, termProgram := internal.GetTermEnv()

	caps := SixelCapabilitiesInfo{
		TerminalType: term,
		InTmux:       os.Getenv("TMUX") != "",
	}

	// 環境変数からチェック
	if checkSixelEnvironment(term, termProgram) {
		caps.Supported = true
		caps.DetectionMethod = "environment"
	}

	// tmuxの特別処理
	if caps.InTmux {
		caps.TmuxSixelReady = checkTmuxSixelSupport()
		if caps.TmuxSixelReady {
			caps.Supported = true
			caps.DetectionMethod = "tmux-sixel"
		} else {
			caps.Supported = false
			return caps
		}
	}

	if !caps.Supported && checkSixelWithLsix() {
		caps.Supported = true
		caps.DetectionMethod = "lsix"
	}

	// インタラクティブ端末の場合は能動的検出
	if !caps.Supported && internal.IsInteractiveTerminal() && detectSixelWithQuery() {
		caps.Supported = true
		caps.DetectionMethod = "query"
	}

	// エンコーダコマンドの確認
	if caps.Supported {
		cmd, args := getSixelEncoderCommand()
		if cmd == "" {
			// エンコーダがない場合はSixelサポートを無効化
			caps.Supported = false
			caps.DetectionMethod = "no-encoder"
		} else {
			caps.EncoderCommand = cmd
			caps.EncoderArgs = args
		}
	}

	return caps
}

func checkSixelEnvironment(term, termProgram string) bool {
	// Sixel対応端末のリスト
	sixelTerminals := []string{
		"xterm", "mlterm", "yaft", "rlogin", "wezterm",
		"foot", "contour", "mintty", "iterm",
	}

	for _, st := range sixelTerminals {
		if strings.Contains(term, st) || strings.Contains(termProgram, st) {
			return true
		}
	}

	return false
}

// checkTmuxSixelSupport tmuxのSixelサポートを確認
func checkTmuxSixelSupport() bool {
	cmd := exec.Command("tmux", "-V")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	version := string(output)

	// tmux 3.3a以降は--enable-sixelでコンパイルされていればサポート
	if strings.Contains(version, "3.3") || strings.Contains(version, "3.4") {
		return true
	}

	// sixel-tmux forkのチェック
	if strings.Contains(version, "sixel") {
		return true
	}

	return false
}

// checkSixelWithLsix lsixコマンドでSixelサポートを検証
func checkSixelWithLsix() bool {
	_, err := exec.LookPath("lsix")
	if err != nil {
		return false
	}

	cmd := exec.Command("lsix", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	outputStr := string(output)

	// lsixはSixel非対応端末でエラーメッセージを出す
	if strings.Contains(outputStr, "does not report having sixel graphics support") {
		return false
	}

	return true
}

// detectSixelWithQuery Device Attributesを使ったSixel検出
func detectSixelWithQuery() bool {
	fmt.Print("\x1b[c")

	reader := bufio.NewReader(os.Stdin)
	responseChan := make(chan string, 1)

	go func() {
		var response strings.Builder
		for {
			ch, err := reader.ReadByte()
			if err != nil {
				break
			}
			response.WriteByte(ch)
			if ch == 'c' {
				break
			}
		}
		responseChan <- response.String()
	}()

	select {
	case response := <-responseChan:
		// レスポンスに'4'が含まれていればSixelサポート
		// フォーマット: ESC[?1;2;...;4;...c
		return strings.Contains(response, ";4") || strings.Contains(response, "?4")
	case <-time.After(500 * time.Millisecond):
		return false
	}
}

// getSixelEncoderCommand Sixel変換コマンドを取得
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