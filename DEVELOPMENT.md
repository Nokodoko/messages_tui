# Messages TUI - Development Status

## Current State
**Builds successfully** - Binary at `./messages-tui` (~15MB)

## What's Implemented

### Core Infrastructure
- Go module with all dependencies resolved (Bubble Tea, Lip Gloss, libgm)
- Makefile with `build`, `run`, `dev`, `deps` targets
- Config loading from `~/.config/messages-tui/config.yaml`
- Session persistence to `~/.config/messages-tui/session.json`

### Client Layer (`internal/client/`)
- `auth.go` - QR code pairing flow using libgm's `StartLogin()`
- `client.go` - Wrapper around libgm with `ListConversations`, `GetMessages`, `SendMessage`, `SendReaction`, `MarkRead`
- `events.go` - Event types and converters from gmproto to internal store types

### UI Layer (`internal/ui/`)
- `app.go` - Root Bubble Tea model with 4 states: Loading, QRPairing, Connected, Error
- `contacts.go` - Left panel with conversation list, search mode (`/`)
- `messages.go` - Center panel with message display, scrolling
- `input.go` - Bottom input bar with inline typing
- `editor.go` - External editor integration via `tea.ExecProcess`
- `styles.go` - Lip Gloss theme with purple/green accent colors

### Key Bindings
- `j/k` - Navigate
- `Tab` - Switch panels
- `Enter` - Select/send
- `e` or `Ctrl+E` - Open external editor (nvim)
- `q` - Quit

## Not Yet Tested
- Actual connection to Google Messages (requires Android phone)
- Real-time message receiving
- Session restoration after restart

## Potential Issues to Watch
1. The libgm API may have changed - conversion functions in `events.go` might need adjustment based on actual response data
2. External editor integration uses `tea.ExecProcess` which suspends TUI - verify it resumes correctly
3. Message content extraction from `MessageInfo.GetMessageContent()` - may need to handle `MediaContent` case too

## Next Steps
1. Test with real Google Messages account
2. Add media message display (images, etc.)
3. Add emoji picker for reactions
4. Add file attachment support
5. Improve error handling and reconnection logic

## Quick Commands
```bash
make deps   # Download dependencies
make build  # Build to ./build/messages-tui
make dev    # Run without building
./messages-tui  # Run built binary
```
