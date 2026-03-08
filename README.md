# yutemal

A Terminal UI (TUI) YouTube Music player written in Go.

## About

yutemal is a terminal-based YouTube Music client that allows you to browse, search, and play music directly from your terminal.

## Features

- 🎵 Stream YouTube Music directly in your terminal
- 🔍 Search for songs, albums, and playlists
- 📋 Browse your YouTube Music library and playlists
- ⌨️ Vim-style keyboard navigation
- 🖱️ Mouse support (click to select/play, wheel scroll, seek via progress bar)
- 🎨 Customizable themes with multiple presets

## Requirements

- Go 1.26.1 or later (managed via [mise](https://mise.jdx.dev/))
- yt-dlp (for downloading audio)
- ffprobe (part of FFmpeg, for audio analysis)
- Linux: `libasound2-dev libdbus-1-dev pkg-config`
- macOS: No additional requirements

## Installation

### Install with Go

```bash
go install github.com/haryoiro/yutemal@latest
```

### Build from source

```bash
# Clone the repository
git clone https://github.com/haryoiro/yutemal
cd yutemal

# Build the binary
./build.sh

# Or manually
go build -o yutemal main.go
```

## Configuration

### Authentication

yutemal requires YouTube Music cookies for authentication. There are three methods:

#### Method 1: Configure browser in config.toml (Recommended)

Set the `browser` (and optionally `browser_profile`) in your config file:

```toml
browser = "chrome"           # "chrome", "chrome-beta", "chrome-canary", or "chromium"
browser_profile = "Default"  # e.g., "Default", "Profile 1", "Profile 2"
```

yutemal will read cookies directly from the specified browser's cookie database.

> **Note**: Browser cookie detection is currently supported on **macOS only**, as it relies on the macOS Keychain to decrypt cookies.

#### Method 2: Automatic Chrome fallback

If no `browser` is configured and `headers.txt` is not found, yutemal will automatically try to read cookies from Chrome (Default profile) as a fallback.

#### Method 3: Manual headers.txt

1. Install a browser extension to export cookies (e.g., "Get cookies.txt LOCALLY" for Chrome/Firefox)
2. Visit music.youtube.com and log in
3. Export cookies in Netscape format
4. Save as `~/.config/yutemal/headers.txt` or `./header.txt`

### Configuration File

Copy the example configuration to get started:

```bash
cp config.example.toml ~/.config/yutemal/config.toml
```

Key configuration options:
- `browser`: Browser to read cookies from (`chrome`, `chrome-beta`, `chrome-canary`, `chromium`)
- `browser_profile`: Browser profile name (e.g., `Default`, `Profile 1`)
- `audio_quality`: Set download quality (low/medium/high/best)
- `theme`: Choose from built-in themes (Tokyo Night Storm, Catppuccin Mocha, Dracula, Nord, Gruvbox Dark)
- `progress_bar_style`: Progress bar style (line/block/gradient)
- `key_bindings`: Customize keyboard shortcuts

## Usage

```bash
# Run yutemal
./yutemal

# Show help and keyboard shortcuts
./yutemal --help

# Show version
./yutemal --version

# Show file locations
./yutemal --files

# Enable debug logging
./yutemal --debug

# Clear all cache data
./yutemal --clear-cache

# Fix database issues
./yutemal --fix-db
```

## Keyboard Shortcuts

### Global Controls
- `Space`: Play/Pause
- `←/→`: Seek backward/forward
- `+/=`: Volume up
- `-/_`: Volume down
- `Ctrl+C` or `Ctrl+D`: Quit application

### Navigation
- `↑/k`: Move up
- `↓/j`: Move down
- `PgUp/PgDn`: Page scroll
- `Enter/l`: Select/Play
- `Esc/Backspace`: Go back

### View Controls
- `Tab`: Next section
- `Shift+Tab`: Previous section
- `f`: Open search
- `h`: Return to home
- `r`: Remove track from playlist

## Mouse Support

- **Left Click**: Select and play items
- **Progress Bar Click**: Seek to position
- **Wheel Scroll**: Navigate through lists

## File Locations

yutemal follows the XDG Base Directory specification:

```
Configuration: ~/.config/yutemal/
Cache:         ~/.cache/yutemal/
Data:          ~/.local/share/yutemal/
```

## Troubleshooting

### Missing Dependencies

If you see errors about missing yt-dlp or ffprobe:

```bash
# macOS
brew install yt-dlp ffmpeg

# Linux
sudo apt install yt-dlp ffmpeg

# Or via pip
pip install yt-dlp
```

### Authentication Issues

1. Ensure you are logged in to YouTube Music in your browser (Chrome, Chrome Beta, Chrome Canary, or Chromium)
2. If using `browser` config, verify the browser name and profile are correct
3. If using `headers.txt`, make sure it exists in the correct location
4. Check that your cookies are not expired
5. Browser cookie detection requires **macOS** — on Linux, use `headers.txt` instead

### Debug Mode

Run with `--debug` flag to enable detailed logging:

```bash
./yutemal --debug
```

Logs are saved to `~/.local/share/yutemal/yutemal.log`

### Clearing Cache

If you experience issues, try clearing the cache:

```bash
./yutemal --clear-cache
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
