# Messages TUI - Implementation Plan

A terminal user interface for Google Messages, similar to weechat/neomutt.

## Tech Stack

- **Language**: Go 1.22+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Protocol**: [libgm](https://pkg.go.dev/go.mau.fi/mautrix-gmessages/pkg/libgm) (reverse-engineered Google Messages protocol)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Messages TUI                            │
├─────────────┬───────────────────────────────────┬───────────────┤
│ Contacts    │          Messages                 │ Details       │
│ Panel       │          Panel                    │ Panel         │
│             │                                   │               │
│ > Alice     │  Alice: Hey!              10:30   │ Alice Smith   │
│   Bob       │  You: Hi there            10:31   │ +1234567890   │
│   Carol     │  Alice: How are you?      10:32   │               │
│   Dave      │  You: [image.png]         10:33   │ Reactions:    │
│             │       👍 Alice                    │ enabled       │
│             │                                   │               │
│             │                                   │               │
├─────────────┴───────────────────────────────────┴───────────────┤
│ [Input: Type a message...]                           [Ctrl+S] 📎│
├─────────────────────────────────────────────────────────────────┤
│ Status: Connected │ RCS: Active │ 5 unread                      │
└─────────────────────────────────────────────────────────────────┘
```

## Core Features

### Phase 1 - MVP
- [ ] QR code pairing with phone (display in terminal)
- [ ] List conversations
- [ ] View messages in a conversation
- [ ] Send text messages
- [ ] Receive messages in real-time

### Phase 2 - Rich Features
- [ ] Message reactions (emoji picker)
- [ ] Send images/GIFs (file picker + preview)
- [ ] Typing indicators
- [ ] Read receipts
- [ ] Message search

### Phase 3 - Polish
- [ ] Vim-style keybindings (j/k navigation, etc.)
- [ ] Configurable themes
- [ ] Desktop notifications
- [ ] Message history persistence/caching
- [ ] Multiple account support

## Key Bindings (weechat-inspired)

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate messages/contacts |
| `Tab` | Switch panels |
| `Enter` | Select conversation / Send message |
| `Ctrl+R` | React to message |
| `Ctrl+A` | Attach file (image/GIF) |
| `Ctrl+G` | GIF picker |
| `/` | Search |
| `q` or `Ctrl+C` | Quit |

## Project Structure

```
messages_tui/
├── cmd/
│   └── messages-tui/
│       └── main.go           # Entry point
├── internal/
│   ├── client/
│   │   ├── client.go         # libgm wrapper
│   │   ├── auth.go           # QR pairing logic
│   │   └── events.go         # Message event handlers
│   ├── ui/
│   │   ├── app.go            # Main Bubble Tea model
│   │   ├── contacts.go       # Contacts panel component
│   │   ├── messages.go       # Messages panel component
│   │   ├── input.go          # Input bar component
│   │   ├── status.go         # Status bar component
│   │   └── styles.go         # Lip Gloss styles
│   ├── store/
│   │   └── store.go          # Local message cache
│   └── config/
│       └── config.go         # User configuration
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea   // TUI framework
    github.com/charmbracelet/lipgloss    // Styling
    github.com/charmbracelet/bubbles     // Pre-built components
    go.mau.fi/mautrix-gmessages/pkg/libgm // Google Messages protocol
    github.com/skip2/go-qrcode           // QR code generation
    github.com/mattn/go-sixel            // Sixel image support (optional)
)
```

## Implementation Order

1. **Scaffold project** - go mod init, directory structure
2. **libgm integration** - Connect, pair via QR, list conversations
3. **Basic TUI** - Conversation list, message view (read-only)
4. **Send messages** - Input handling, message sending
5. **Real-time updates** - Event handling for incoming messages
6. **Reactions** - Emoji picker, reaction sending
7. **Media** - Image/GIF sending with file picker
8. **Polish** - Keybindings, themes, notifications
