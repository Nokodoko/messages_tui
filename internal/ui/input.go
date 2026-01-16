package ui

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputMode represents the current mode of the input (like vim)
type InputMode int

const (
	ModeInsert InputMode = iota
	ModeNormal
)

// InputKeyMap defines the key bindings for the input component
type InputKeyMap struct {
	Send       key.Binding
	AttachFile key.Binding
}

// DefaultInputKeyMap returns the default key bindings
func DefaultInputKeyMap() InputKeyMap {
	return InputKeyMap{
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		AttachFile: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "attach"),
		),
	}
}

// SendMessageMsg is sent when the user wants to send a message
type SendMessageMsg struct {
	Content string
}

// OpenEditorMsg is sent when the user wants to open the external editor
type OpenEditorMsg struct {
	InitialContent string
}

// AttachFileMsg is sent when the user wants to attach a file
type AttachFileMsg struct{}

// PendingAction represents a vim command waiting for additional input
type PendingAction int

const (
	PendingNone PendingAction = iota
	PendingFindForward  // f - waiting for char to find forward
	PendingFindBackward // F - waiting for char to find backward
	PendingChange       // c - waiting for motion (w, e, $, etc.)
	PendingDelete       // d - waiting for motion (w, e, $, etc.)
)

// InputModel represents the message input component
type InputModel struct {
	textInput     textinput.Model
	draftContent  string // Stores full multiline content from editor
	width         int
	focused       bool
	styles        *Styles
	keyMap        InputKeyMap
	sending       bool          // Show "Sending..." indicator
	mode          InputMode     // Current vim mode (insert/normal)
	pendingAction PendingAction // Pending vim command waiting for char
	lastFindChar  byte          // Last character used with f/F
	lastFindDir   int           // 1 = forward (f), -1 = backward (F)
}

// NewInputModel creates a new input model
func NewInputModel(styles *Styles) InputModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message... (Esc for normal mode)"
	ti.CharLimit = 5000
	ti.Width = 50

	return InputModel{
		textInput: ti,
		styles:    styles,
		keyMap:    DefaultInputKeyMap(),
		mode:      ModeInsert, // Start in insert mode
	}
}

// Init initializes the input model
func (m InputModel) Init() tea.Cmd {
	return nil
}

// MessageSentNotifyMsg is sent to clear the sending indicator
type MessageSentNotifyMsg struct{}

// MessageFailedNotifyMsg is sent when message sending fails
type MessageFailedNotifyMsg struct{}

// Update handles messages for the input component
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case MessageSentNotifyMsg:
		m.sending = false
		return m, nil

	case MessageFailedNotifyMsg:
		m.sending = false
		return m, nil

	case tea.KeyMsg:
		if m.focused {
			log.Printf("Input: KeyMsg received, key=%q, mode=%d (0=insert, 1=normal)", msg.String(), m.mode)
			// Handle mode-specific keys
			if m.mode == ModeNormal {
				return m.handleNormalMode(msg)
			}
			return m.handleInsertMode(msg)
		}
	}

	// Update the text input only in insert mode
	if m.focused && m.mode == ModeInsert {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleInsertMode handles keys in insert mode
func (m InputModel) handleInsertMode(msg tea.KeyMsg) (InputModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		// Switch to normal mode
		m.mode = ModeNormal
		m.textInput.Blur()
		return m, nil

	case tea.KeyEnter:
		// Send message
		log.Printf("Input: Enter pressed in insert mode")
		log.Printf("Input: draftContent=%q, textInput.Value=%q", m.draftContent, m.textInput.Value())
		content := strings.TrimSpace(m.draftContent)
		if content == "" {
			content = strings.TrimSpace(m.textInput.Value())
		}
		log.Printf("Input: Final content=%q", content)
		if content != "" {
			m.textInput.Reset()
			m.draftContent = ""
			m.sending = true
			log.Printf("Input: Sending message with content length %d", len(content))
			return m, func() tea.Msg {
				return SendMessageMsg{Content: content}
			}
		}
		log.Printf("Input: No content to send")
		return m, nil

	case tea.KeyCtrlA:
		// Attach file
		return m, func() tea.Msg {
			return AttachFileMsg{}
		}
	}

	// Let textinput handle other keys
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleNormalMode handles keys in normal mode (vim-like)
func (m InputModel) handleNormalMode(msg tea.KeyMsg) (InputModel, tea.Cmd) {
	// Handle pending actions first (f/F waiting for character)
	if m.pendingAction != PendingNone {
		return m.handlePendingAction(msg)
	}

	switch msg.String() {
	case "i":
		// Enter insert mode at cursor
		m.mode = ModeInsert
		m.textInput.Focus()
		return m, nil

	case "a":
		// Enter insert mode after cursor
		m.mode = ModeInsert
		m.textInput.Focus()
		// Move cursor right by one (append mode)
		pos := m.textInput.Position()
		m.textInput.SetCursor(pos + 1)
		return m, nil

	case "A":
		// Enter insert mode at end of line
		m.mode = ModeInsert
		m.textInput.Focus()
		m.textInput.SetCursor(len(m.textInput.Value()))
		return m, nil

	case "I":
		// Enter insert mode at beginning of line
		m.mode = ModeInsert
		m.textInput.Focus()
		m.textInput.SetCursor(0)
		return m, nil

	case "v":
		// Open external editor (like vim's v in readline)
		currentContent := m.draftContent
		if currentContent == "" {
			currentContent = m.textInput.Value()
		}
		return m, func() tea.Msg {
			return OpenEditorMsg{InitialContent: currentContent}
		}

	case "d":
		// d - wait for motion (dw, de, d$, etc.) or dd to clear line
		m.pendingAction = PendingDelete
		return m, nil

	case "D":
		// Delete from cursor to end of line
		m = m.deleteToEndOfLine()
		return m, nil

	case "c":
		// c - wait for motion (cw, ce, c$, etc.)
		m.pendingAction = PendingChange
		return m, nil

	case "C":
		// Change from cursor to end of line (delete to end + insert mode)
		m = m.deleteToEndOfLine()
		m.mode = ModeInsert
		m.textInput.Focus()
		return m, nil

	case "f":
		// Find character forward - wait for next char
		m.pendingAction = PendingFindForward
		return m, nil

	case "F":
		// Find character backward - wait for next char
		m.pendingAction = PendingFindBackward
		return m, nil

	case ";":
		// Repeat last find in same direction
		if m.lastFindChar != 0 {
			m = m.repeatFind(m.lastFindDir)
		}
		return m, nil

	case ",":
		// Repeat last find in opposite direction
		if m.lastFindChar != 0 {
			m = m.repeatFind(-m.lastFindDir)
		}
		return m, nil

	case "0":
		// Move to beginning of line
		m.textInput.SetCursor(0)
		return m, nil

	case "$":
		// Move to end of line
		m.textInput.SetCursor(len(m.textInput.Value()))
		return m, nil

	case "h":
		// Move left
		pos := m.textInput.Position()
		if pos > 0 {
			m.textInput.SetCursor(pos - 1)
		}
		return m, nil

	case "l":
		// Move right
		pos := m.textInput.Position()
		if pos < len(m.textInput.Value()) {
			m.textInput.SetCursor(pos + 1)
		}
		return m, nil

	case "w":
		// Move to next word
		m = m.moveToNextWord()
		return m, nil

	case "b":
		// Move to previous word
		m = m.moveToPrevWord()
		return m, nil

	case "e":
		// Move to end of word
		m = m.moveToEndOfWord()
		return m, nil

	case "x":
		// Delete character under cursor
		m = m.deleteCharAtCursor()
		return m, nil

	case "enter":
		// Send message (also works in normal mode)
		content := strings.TrimSpace(m.draftContent)
		if content == "" {
			content = strings.TrimSpace(m.textInput.Value())
		}
		if content != "" {
			m.textInput.Reset()
			m.draftContent = ""
			m.sending = true
			return m, func() tea.Msg {
				return SendMessageMsg{Content: content}
			}
		}
		return m, nil

	case "esc":
		// Cancel any pending action
		m.pendingAction = PendingNone
		return m, nil
	}

	return m, nil
}

// handlePendingAction handles the second character for f/F/c/d commands
func (m InputModel) handlePendingAction(msg tea.KeyMsg) (InputModel, tea.Cmd) {
	char := msg.String()

	// Cancel on escape
	if char == "esc" {
		m.pendingAction = PendingNone
		return m, nil
	}

	val := m.textInput.Value()
	pos := m.textInput.Position()

	switch m.pendingAction {
	case PendingFindForward:
		// Only handle single character inputs for f
		if len(char) != 1 {
			m.pendingAction = PendingNone
			return m, nil
		}
		targetChar := char[0]
		m.lastFindChar = targetChar
		m.lastFindDir = 1 // forward
		// Find character forward from cursor
		for i := pos + 1; i < len(val); i++ {
			if val[i] == targetChar {
				m.textInput.SetCursor(i)
				break
			}
		}

	case PendingFindBackward:
		// Only handle single character inputs for F
		if len(char) != 1 {
			m.pendingAction = PendingNone
			return m, nil
		}
		targetChar := char[0]
		m.lastFindChar = targetChar
		m.lastFindDir = -1 // backward
		// Find character backward from cursor
		for i := pos - 1; i >= 0; i-- {
			if val[i] == targetChar {
				m.textInput.SetCursor(i)
				break
			}
		}

	case PendingChange:
		// Handle change motions: cw, ce, c$, cc
		m.pendingAction = PendingNone
		switch char {
		case "w", "e":
			// Change word - delete to end of word and enter insert mode
			m = m.deleteToEndOfWord()
			m.mode = ModeInsert
			m.textInput.Focus()
			return m, nil
		case "$":
			// Change to end of line
			m = m.deleteToEndOfLine()
			m.mode = ModeInsert
			m.textInput.Focus()
			return m, nil
		case "c":
			// cc - change entire line
			m.textInput.Reset()
			m.draftContent = ""
			m.mode = ModeInsert
			m.textInput.Focus()
			return m, nil
		case "0":
			// c0 - change to beginning of line
			m = m.deleteToBeginningOfLine()
			m.mode = ModeInsert
			m.textInput.Focus()
			return m, nil
		}
		return m, nil

	case PendingDelete:
		// Handle delete motions: dw, de, d$, dd
		m.pendingAction = PendingNone
		switch char {
		case "w", "e":
			// Delete word
			m = m.deleteToEndOfWord()
			return m, nil
		case "$":
			// Delete to end of line
			m = m.deleteToEndOfLine()
			return m, nil
		case "d":
			// dd - delete entire line
			m.textInput.Reset()
			m.draftContent = ""
			return m, nil
		case "0":
			// d0 - delete to beginning of line
			m = m.deleteToBeginningOfLine()
			return m, nil
		}
		return m, nil
	}

	m.pendingAction = PendingNone
	return m, nil
}

// moveToNextWord moves cursor to the start of the next word
func (m InputModel) moveToNextWord() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	// Skip current word
	for pos < len(val) && val[pos] != ' ' {
		pos++
	}
	// Skip spaces
	for pos < len(val) && val[pos] == ' ' {
		pos++
	}
	m.textInput.SetCursor(pos)
	return m
}

// moveToPrevWord moves cursor to the start of the previous word
func (m InputModel) moveToPrevWord() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	// Skip spaces before cursor
	for pos > 0 && val[pos-1] == ' ' {
		pos--
	}
	// Skip to start of word
	for pos > 0 && val[pos-1] != ' ' {
		pos--
	}
	m.textInput.SetCursor(pos)
	return m
}

// deleteCharAtCursor deletes the character at the cursor position
func (m InputModel) deleteCharAtCursor() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	if pos < len(val) {
		newVal := val[:pos] + val[pos+1:]
		m.textInput.SetValue(newVal)
		m.textInput.SetCursor(pos)
	}
	return m
}

// deleteToEndOfLine deletes from cursor to end of line (D command)
func (m InputModel) deleteToEndOfLine() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	if pos < len(val) {
		m.textInput.SetValue(val[:pos])
		m.textInput.SetCursor(pos)
	}
	return m
}

// deleteToBeginningOfLine deletes from cursor to beginning of line
func (m InputModel) deleteToBeginningOfLine() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	if pos > 0 {
		m.textInput.SetValue(val[pos:])
		m.textInput.SetCursor(0)
	}
	return m
}

// repeatFind repeats the last f/F find in the given direction (1=forward, -1=backward)
func (m InputModel) repeatFind(dir int) InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	if dir > 0 {
		// Find forward
		for i := pos + 1; i < len(val); i++ {
			if val[i] == m.lastFindChar {
				m.textInput.SetCursor(i)
				break
			}
		}
	} else {
		// Find backward
		for i := pos - 1; i >= 0; i-- {
			if val[i] == m.lastFindChar {
				m.textInput.SetCursor(i)
				break
			}
		}
	}
	return m
}

// deleteToEndOfWord deletes from cursor to end of current word
func (m InputModel) deleteToEndOfWord() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()
	endPos := pos

	// Skip current word characters
	for endPos < len(val) && val[endPos] != ' ' {
		endPos++
	}
	// Also skip trailing space
	for endPos < len(val) && val[endPos] == ' ' {
		endPos++
	}

	if endPos > pos {
		newVal := val[:pos] + val[endPos:]
		m.textInput.SetValue(newVal)
		m.textInput.SetCursor(pos)
	}
	return m
}

// moveToEndOfWord moves cursor to the end of the current/next word
func (m InputModel) moveToEndOfWord() InputModel {
	val := m.textInput.Value()
	pos := m.textInput.Position()

	// Skip current position
	if pos < len(val) {
		pos++
	}

	// Skip spaces
	for pos < len(val) && val[pos] == ' ' {
		pos++
	}

	// Move to end of word
	for pos < len(val) && val[pos] != ' ' {
		pos++
	}

	// Position at last char of word, not after it
	if pos > 0 && (pos >= len(val) || val[pos] == ' ') {
		pos--
	}

	m.textInput.SetCursor(pos)
	return m
}


// View renders the input component
func (m InputModel) View() string {
	style := m.styles.Input
	if m.focused {
		// Change border color based on mode
		if m.mode == ModeNormal {
			// Purple border for normal mode
			style = m.styles.InputFocused // Already purple (PrimaryColor)
		} else {
			// Cyan border for insert mode
			style = m.styles.InputFocused.BorderForeground(CyanColor)
		}
	}

	// Set the prompt style
	m.textInput.PromptStyle = m.styles.InputPrompt
	m.textInput.TextStyle = m.styles.ContactName
	m.textInput.PlaceholderStyle = m.styles.InputPlaceholder

	// Mode indicator and placeholder
	var modeIndicator string
	if m.mode == ModeNormal {
		modeIndicator = m.styles.ContactUnread.Render("[N] ")
		m.textInput.Placeholder = "'i' for insert mode"
	} else {
		modeIndicator = lipgloss.NewStyle().Foreground(CyanColor).Bold(true).Render("[I] ")
		m.textInput.Placeholder = "Type a message... (Esc for normal mode)"
	}

	inputView := m.textInput.View()

	// Show sending indicator or mode
	rightIndicator := ""
	if m.sending {
		rightIndicator = m.styles.ContactUnread.Render(" Sending...")
	}

	// Calculate spacing for right-aligned indicator
	contentLen := len(modeIndicator) + len(inputView) + len(rightIndicator)
	spacing := ""
	if contentLen < m.width-4 && rightIndicator != "" {
		spacing = strings.Repeat(" ", m.width-4-contentLen)
	}

	fullView := modeIndicator + inputView + spacing + rightIndicator

	return style.Width(m.width).Render(fullView)
}

// SetWidth sets the input width
func (m *InputModel) SetWidth(width int) {
	m.width = width
	m.textInput.Width = width - 8 // Account for padding, borders, and mode indicator [N]
}

// SetFocused sets the focus state
func (m *InputModel) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		// Start in insert mode when focused
		m.mode = ModeInsert
		m.textInput.Focus()
	} else {
		m.textInput.Blur()
	}
}

// Focus focuses the input
func (m *InputModel) Focus() tea.Cmd {
	m.focused = true
	return m.textInput.Focus()
}

// Blur removes focus from the input
func (m *InputModel) Blur() {
	m.focused = false
	m.textInput.Blur()
}

// Value returns the current input value
func (m InputModel) Value() string {
	return m.textInput.Value()
}

// SetValue sets the input value, handling multiline content
func (m *InputModel) SetValue(value string) {
	m.draftContent = value
	// Show preview in textInput (first line or truncated)
	if strings.Contains(value, "\n") {
		lines := strings.Split(value, "\n")
		lineCount := len(lines)
		preview := lines[0]
		if len(preview) > 30 {
			preview = preview[:30] + "..."
		}
		m.textInput.SetValue(preview + " [+" + fmt.Sprintf("%d", lineCount-1) + " lines]")
	} else {
		m.textInput.SetValue(value)
	}
}

// Reset clears the input
func (m *InputModel) Reset() {
	m.textInput.Reset()
	m.draftContent = ""
}

// IsFocused returns whether the input is focused
func (m InputModel) IsFocused() bool {
	return m.focused
}

// Mode returns the current input mode
func (m InputModel) Mode() InputMode {
	return m.mode
}
