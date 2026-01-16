package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/n0ko/messages-tui/internal/store"
)

// MessagesKeyMap defines the key bindings for the messages panel
type MessagesKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	React    key.Binding
}

// DefaultMessagesKeyMap returns the default key bindings
func DefaultMessagesKeyMap() MessagesKeyMap {
	return MessagesKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("gg/home", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "bottom"),
		),
		React: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "react"),
		),
	}
}

// MessagesModel represents the messages panel
type MessagesModel struct {
	messages       []*store.Message
	conversationID string
	selected       int
	offset         int
	width          int
	height         int
	focused        bool
	styles         *Styles
	keyMap         MessagesKeyMap
	lastKeyWasG    bool // Track if last key was 'g' for gg combo
}

// NewMessagesModel creates a new messages panel model
func NewMessagesModel(styles *Styles) MessagesModel {
	return MessagesModel{
		messages: []*store.Message{},
		styles:   styles,
		keyMap:   DefaultMessagesKeyMap(),
	}
}

// Init initializes the messages model
func (m MessagesModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the panel
func (m MessagesModel) Update(msg tea.Msg) (MessagesModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle gg combo for going to top
		if msg.String() == "g" {
			if m.lastKeyWasG {
				// gg pressed - go to top
				m.selected = 0
				m.offset = 0
				m.lastKeyWasG = false
				return m, nil
			}
			m.lastKeyWasG = true
			return m, nil
		}
		m.lastKeyWasG = false

		switch {
		case key.Matches(msg, m.keyMap.Up):
			if m.selected > 0 {
				m.selected--
				if m.selected < m.offset {
					m.offset = m.selected
				}
			}

		case key.Matches(msg, m.keyMap.Down):
			if m.selected < len(m.messages)-1 {
				m.selected++
				visibleItems := m.visibleItemCount()
				if m.selected >= m.offset+visibleItems {
					m.offset = m.selected - visibleItems + 1
				}
			}

		case key.Matches(msg, m.keyMap.PageUp):
			pageSize := m.visibleItemCount()
			m.selected = max(0, m.selected-pageSize)
			m.offset = max(0, m.offset-pageSize)

		case key.Matches(msg, m.keyMap.PageDown):
			pageSize := m.visibleItemCount()
			maxSelect := len(m.messages) - 1
			m.selected = min(maxSelect, m.selected+pageSize)
			m.offset = min(max(0, len(m.messages)-pageSize), m.offset+pageSize)

		case key.Matches(msg, m.keyMap.Top):
			// Home - go to top
			m.selected = 0
			m.offset = 0

		case key.Matches(msg, m.keyMap.Bottom):
			// G/End - go to bottom
			if len(m.messages) > 0 {
				m.selected = len(m.messages) - 1
				m.offset = max(0, len(m.messages)-m.visibleItemCount())
			}
		}
	}

	return m, nil
}

// View renders the messages panel
func (m MessagesModel) View() string {
	var b strings.Builder

	// Title
	title := "Messages"
	if m.conversationID != "" {
		title = fmt.Sprintf("Messages (%d)", len(m.messages))
	}
	b.WriteString(m.styles.PanelTitleText.Render(title))
	b.WriteString("\n")

	if len(m.messages) == 0 {
		// Show empty state
		emptyMsg := "No messages"
		if m.conversationID == "" {
			emptyMsg = "Select a conversation"
		}
		b.WriteString("\n")
		b.WriteString(m.styles.ContactPreview.Render(emptyMsg))
	} else {
		// Calculate available height
		availableHeight := m.height - 3

		// Render messages
		visibleCount := 0
		for i := m.offset; i < len(m.messages) && visibleCount < availableHeight; i++ {
			msg := m.messages[i]
			rendered := m.renderMessage(msg, i == m.selected)
			lines := strings.Count(rendered, "\n") + 1
			if visibleCount+lines > availableHeight {
				break
			}
			b.WriteString(rendered)
			b.WriteString("\n")
			visibleCount += lines
		}
	}

	// Apply panel style
	style := m.styles.Panel
	if m.focused {
		style = m.styles.PanelActive
	}

	return style.Width(m.width).Height(m.height).Render(b.String())
}

// renderMessage renders a single message
func (m MessagesModel) renderMessage(msg *store.Message, selected bool) string {
	maxWidth := m.width - 6 // Account for padding and borders

	// Determine message style based on sender
	var msgStyle lipgloss.Style
	if msg.IsFromMe {
		msgStyle = m.styles.MessageSent
	} else {
		msgStyle = m.styles.MessageReceived
	}

	// Highlight if selected
	if selected {
		msgStyle = msgStyle.BorderForeground(PrimaryColor).
			Border(lipgloss.NormalBorder(), false, false, false, true)
	}

	// Format content
	content := msg.Content
	if len(content) > maxWidth*3 {
		content = content[:maxWidth*3-3] + "..."
	}

	// Wrap content to max width
	content = wrapText(content, maxWidth-4)

	// Format time
	timeStr := msg.Timestamp.Format("15:04")

	// Format status for sent messages
	statusStr := ""
	if msg.IsFromMe {
		switch msg.Status {
		case "delivered":
			statusStr = " ✓"
		case "read":
			statusStr = " ✓✓"
		case "failed":
			statusStr = " ✗"
		}
	}

	// Build the message
	var result strings.Builder

	// Sender name for received messages in groups
	if !msg.IsFromMe && msg.SenderName != "" {
		result.WriteString(m.styles.MessageSender.Render(msg.SenderName))
		result.WriteString("\n")
	}

	// Message content
	result.WriteString(content)

	// Time and status on the same line
	footer := m.styles.MessageTime.Render(timeStr)
	if statusStr != "" {
		if msg.Status == "read" {
			footer += m.styles.MessageStatusRead.Render(statusStr)
		} else {
			footer += m.styles.MessageStatus.Render(statusStr)
		}
	}
	result.WriteString("\n")
	result.WriteString(footer)

	// Apply alignment
	renderedMsg := msgStyle.MaxWidth(maxWidth).Render(result.String())

	if msg.IsFromMe {
		// Right-align sent messages
		padding := maxWidth - lipgloss.Width(renderedMsg)
		if padding > 0 {
			renderedMsg = strings.Repeat(" ", padding) + renderedMsg
		}
	}

	return renderedMsg
}

// visibleItemCount returns approximate number of visible messages
func (m MessagesModel) visibleItemCount() int {
	return (m.height - 3) / 3 // Rough estimate: 3 lines per message
}

// SetMessages updates the message list
func (m *MessagesModel) SetMessages(conversationID string, msgs []*store.Message) {
	m.conversationID = conversationID
	m.messages = msgs

	// Scroll to bottom on new conversation
	if len(msgs) > 0 {
		m.selected = len(msgs) - 1
		m.offset = max(0, len(msgs)-m.visibleItemCount())
	} else {
		m.selected = 0
		m.offset = 0
	}
}

// AddMessage adds a new message and scrolls to it
func (m *MessagesModel) AddMessage(msg *store.Message) {
	if msg.ConversationID != m.conversationID {
		return
	}
	m.messages = append(m.messages, msg)
	// Auto-scroll to new message
	m.selected = len(m.messages) - 1
	m.offset = max(0, len(m.messages)-m.visibleItemCount())
}

// SetSize sets the panel dimensions
func (m *MessagesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets the focus state
func (m *MessagesModel) SetFocused(focused bool) {
	m.focused = focused
}

// SelectedMessage returns the currently selected message
func (m MessagesModel) SelectedMessage() *store.Message {
	if m.selected >= 0 && m.selected < len(m.messages) {
		return m.messages[m.selected]
	}
	return nil
}

// Clear clears the messages
func (m *MessagesModel) Clear() {
	m.messages = nil
	m.conversationID = ""
	m.selected = 0
	m.offset = 0
}

// wrapText wraps text to the specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		lineLen := 0
		for j, word := range words {
			wordLen := len(word)
			if j > 0 && lineLen+1+wordLen > width {
				result.WriteString("\n")
				lineLen = 0
			} else if j > 0 {
				result.WriteString(" ")
				lineLen++
			}
			result.WriteString(word)
			lineLen += wordLen
		}
	}

	return result.String()
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
