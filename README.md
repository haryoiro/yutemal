# yutemal - YouTube Music AT Terminal 🎵

A Terminal UI (TUI) YouTube Music player written in Go.

## About

yutemal is a terminal-based YouTube Music client that allows you to browse, search, and play music directly from your terminal. This project is inspired by and based on the excellent work of [ytermusic](https://github.com/ccgauche/ytermusic) by ccgauche, reimplemented in Go with a focus on performance and cross-platform compatibility.

## Features

- 🎵 Stream YouTube Music directly in your terminal
- 🔍 Search for songs, albums, and playlists
- 📋 Browse your YouTube Music library and playlists
- ⌨️ Vim-style keyboard navigation
- 🎨 Customizable themes
- 💾 Local caching for offline playback
- 🚀 Lightweight and fast

## Requirements

- Go 1.23 or later
- yt-dlp (for downloading audio)
- Linux: `libasound2-dev libdbus-1-dev pkg-config`
- macOS: No additional requirements
- Windows: No additional requirements

## Installation

### Build from source

```bash
# Clone the repository
git clone https://github.com/haryoiro/yutemal
cd yutemal

# Build the binary
./build.sh

# Or manually
go build -o yutemal cmd/yutemal/main.go
```

## Configuration

yutemal requires YouTube Music cookies for authentication. You can export cookies from your browser using a browser extension and save them in the appropriate format.

## Usage

```bash
# Run yutemal
./yutemal

# Show help
./yutemal --help
```

## Keyboard Shortcuts

- `j/k` or `↓/↑`: Navigate up/down
- `Enter`: Select/Play
- `Space`: Pause/Resume
- `q`: Quit
- `/`: Search
- `h/l`: Navigate between views

## Project Structure

```
yutemal/
├── cmd/
│   └── yutemal/
│       └── main.go          # Entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── database/            # Local music database
│   ├── logger/              # Logging utilities
│   ├── player/              # Audio playback engine
│   ├── structures/          # Core data structures
│   ├── systems/             # Core systems (player, download, API)
│   └── ui/                  # Terminal UI components
└── pkg/
    └── ytapi/               # YouTube Music API client
```

## Acknowledgments

This project is a Go implementation inspired by [ytermusic](https://github.com/ccgauche/ytermusic), originally written in Rust by ccgauche. We are grateful for their innovative work in creating a terminal-based YouTube Music player, which served as the foundation and inspiration for yutemal.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Disclaimer

This project is not affiliated with YouTube or Google. It uses the YouTube Music API in accordance with their terms of service.
