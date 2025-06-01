package detect

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
)

// TerminalCapabilities 端末の詳細な機能情報
type TerminalCapabilities struct {
	SupportsKittyGraphics bool
	SupportsKittyKeyboard bool
	TerminalName          string
	IsTmux                bool
	IsSSH                 bool
}

func Capabilities() TerminalCapabilities {
	term, termProgram := internal.GetTermEnv()

	caps := TerminalCapabilities{
		TerminalName: term,
		IsTmux:       os.Getenv("TMUX") != "",
		IsSSH:        os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "",
	}

	if internal.IsKittyTerminal(term, termProgram) || os.Getenv("KITTY_WINDOW_ID") != "" {
		caps.SupportsKittyGraphics = true
		caps.SupportsKittyKeyboard = true

		// tmux内ではグラフィックスを無効化
		if caps.IsTmux {
			caps.SupportsKittyGraphics = false
		}
	}

	return caps
}

// KittyProtocolWithQuery エスケープシーケンスを使った能動的検出
// 警告: 端末状態を変更するため慎重に使用すること
func KittyProtocolWithQuery() bool {
	if !internal.IsInteractiveTerminal() {
		return false
	}

	fmt.Print("\x1b_Gi=1,a=q;\x1b\\")

	reader := bufio.NewReader(os.Stdin)
	responseChan := make(chan string, 1)

	go func() {
		response, _ := reader.ReadString('\\')
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		return strings.Contains(response, "_G") && strings.Contains(response, "OK")
	case <-time.After(100 * time.Millisecond):
		return false
	}
}
