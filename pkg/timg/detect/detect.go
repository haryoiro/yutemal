package detect

import (
	"os"
	"os/exec"
	"strings"

	"github.com/haryoiro/yutemal/pkg/timg/internal"
	"github.com/haryoiro/yutemal/pkg/timg/protocol"
)

// Auto 利用可能な最適なグラフィックプロトコルを自動検出
func Auto() protocol.Protocol {
	// Kittyを優先的にチェック（最も機能が豊富）
	if Capabilities().SupportsKittyGraphics {
		return protocol.New(protocol.TypeKitty)
	}

	_, termProgram := internal.GetTermEnv()

	if strings.Contains(termProgram, "iterm") || os.Getenv("ITERM_SESSION_ID") != "" {
		return protocol.New(protocol.TypeITerm2)
	}

	if SixelCapabilities().Supported {
		return protocol.New(protocol.TypeSixel)
	}

	if checkTerminalGraphicsSupport() {
		return protocol.New(protocol.TypeTerminalGraphics)
	}

	return nil
}

func checkTerminalGraphicsSupport() bool {
	_, err := exec.LookPath("w3m-img")
	return err == nil
}
