# Messages TUI

A terminal UI for Google Messages, similar to weechat/neomutt.

## Features

- **QR Code Pairing**: Scan QR code with your Android phone to connect
- **Three-Panel Layout**: Contacts | Messages | Input
- **Vim-like Navigation**: Use `j/k` to navigate, `Tab` to switch panels
- **External Editor Support**: Press `e` to compose messages in nvim (or your `$EDITOR`)
- **Session Persistence**: Credentials saved to `~/.config/messages-tui/session.json`
- **Real-time Updates**: Receive messages instantly

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/n0ko/messages-tui.git
cd messages-tui

# Install dependencies and build
make deps
make build

# Run
./build/messages-tui
```

### Requirements

- Go 1.22+
- An Android phone with Google Messages installed

## Usage

### First Launch

1. Run `messages-tui`
2. A QR code will appear in the terminal
3. On your Android phone:
   - Open Google Messages
   - Tap ⋮ (menu) → Device Pairing → QR Scanner
   - Scan the QR code
4. Once paired, your conversations will load automatically

### Key Bindings

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate messages/contacts |
| `Tab` | Switch panels |
| `Shift+Tab` | Switch panels (reverse) |
| `Enter` | Select conversation / Send message |
| `e` or `Ctrl+E` | Compose in external editor |
| `/` | Search conversations |
| `q` or `Ctrl+C` | Quit |

### External Editor

When you press `e` or `Ctrl+E`, your configured editor (defaults to `$EDITOR` or nvim) opens with a temporary file. Write your message, then:

- Save and quit (`:wq` in vim) → Message is sent
- Quit without saving (`:q!` in vim) → Message is cancelled

## Configuration

Configuration is stored in `~/.config/messages-tui/config.yaml`:

```yaml
# Editor to use for composing messages
editor: nvim

# Additional arguments to pass to the editor
editor_args: []

# Theme settings
theme:
  primary_color: "#7C3AED"
  secondary_color: "#10B981"
```

## File Locations

- **Config**: `~/.config/messages-tui/config.yaml`
- **Session**: `~/.config/messages-tui/session.json`
- **Logs**: `~/.config/messages-tui/messages-tui.log`

## Architecture

```
messages-tui/
├── cmd/messages-tui/main.go    # Entry point
├── internal/
│   ├── client/                 # libgm wrapper
│   │   ├── client.go           # Connection management
│   │   ├── auth.go             # QR pairing
│   │   └── events.go           # Message handlers
│   ├── ui/                     # Bubble Tea components
│   │   ├── app.go              # Root model
│   │   ├── contacts.go         # Left panel
│   │   ├── messages.go         # Center panel
│   │   ├── input.go            # Message input
│   │   ├── editor.go           # External editor
│   │   └── styles.go           # Lip Gloss themes
│   ├── store/store.go          # Session/message cache
│   └── config/config.go        # User configuration
├── go.mod
└── Makefile
```

## Tech Stack

- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Google Messages Protocol**: [mautrix-gmessages/libgm](https://go.mau.fi/mautrix-gmessages)
- **QR Code**: [go-qrcode](https://github.com/skip2/go-qrcode)

## Troubleshooting

### Session Expired

If your session expires, delete the session file and re-pair:

```bash
rm ~/.config/messages-tui/session.json
messages-tui
```

### Connection Issues

Check the log file for details:

```bash
tail -f ~/.config/messages-tui/messages-tui.log
```

## License

MIT
