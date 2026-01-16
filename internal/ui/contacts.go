package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/n0ko/messages-tui/internal/store"
)

// ContactsKeyMap defines the key bindings for the contacts panel
type ContactsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Select  key.Binding
	Search  key.Binding
}

// DefaultContactsKeyMap returns the default key bindings
func DefaultContactsKeyMap() ContactsKeyMap {
	return ContactsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
	}
}

// ContactsModel represents the contacts/conversations panel
type ContactsModel struct {
	conversations []*store.Conversation
	selected      int
	offset        int
	width         int
	height        int
	focused       bool
	styles        *Styles
	keyMap        ContactsKeyMap
	searchMode    bool
	searchQuery   string
	lastKeyWasG   bool // Track if last key was 'g' for gg combo
}

// NewContactsModel creates a new contacts panel model
func NewContactsModel(styles *Styles) ContactsModel {
	return ContactsModel{
		conversations: []*store.Conversation{},
		selected:      0,
		offset:        0,
		styles:        styles,
		keyMap:        DefaultContactsKeyMap(),
	}
}

// Init initializes the contacts model
func (m ContactsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the contacts panel
func (m ContactsModel) Update(msg tea.Msg) (ContactsModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchMode {
			return m.handleSearchInput(msg)
		}

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
			if m.selected < len(m.conversations)-1 {
				m.selected++
				visibleItems := m.visibleItemCount()
				if m.selected >= m.offset+visibleItems {
					m.offset = m.selected - visibleItems + 1
				}
			}

		case key.Matches(msg, m.keyMap.Bottom):
			// G - go to bottom
			if len(m.conversations) > 0 {
				m.selected = len(m.conversations) - 1
				visibleItems := m.visibleItemCount()
				if m.selected >= visibleItems {
					m.offset = m.selected - visibleItems + 1
				}
			}

		case key.Matches(msg, m.keyMap.Search):
			m.searchMode = true
			m.searchQuery = ""
		}
	}

	return m, nil
}

// handleSearchInput handles input when in search mode
func (m ContactsModel) handleSearchInput(msg tea.KeyMsg) (ContactsModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.searchMode = false
		m.searchQuery = ""
		m.selected = 0
		m.offset = 0
	case tea.KeyEnter:
		m.searchMode = false
		// Keep current selection on the matched item
	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.selected = 0
			m.offset = 0
		}
	case tea.KeyCtrlU:
		// Clear entire line
		m.searchQuery = ""
		m.selected = 0
		m.offset = 0
	case tea.KeyCtrlW:
		// Delete word backward
		m.searchQuery = deleteWordBackward(m.searchQuery)
		m.selected = 0
		m.offset = 0
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.selected = 0
		m.offset = 0
	}
	return m, nil
}

// deleteWordBackward removes the last word from the string
func deleteWordBackward(s string) string {
	if s == "" {
		return ""
	}

	// Trim trailing spaces first
	end := len(s)
	for end > 0 && s[end-1] == ' ' {
		end--
	}

	// Find start of last word
	start := end
	for start > 0 && s[start-1] != ' ' {
		start--
	}

	return s[:start]
}

// View renders the contacts panel
func (m ContactsModel) View() string {
	var b strings.Builder

	// Title
	title := "Conversations"
	titleStyle := m.styles.PanelTitleText
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	// Calculate available height for items
	// Reserve space for title (1) + search bar (1 if in search mode or always show)
	searchBarHeight := 1
	availableHeight := m.height - 4 - searchBarHeight // title, borders, search bar

	// Filter conversations if searching
	conversations := m.getFilteredConversations()

	// Render conversation items
	visibleCount := 0
	linesUsed := 0
	for i := m.offset; i < len(conversations) && linesUsed < availableHeight; i++ {
		conv := conversations[i]
		item := m.renderConversationItem(conv, i == m.selected)
		itemLines := strings.Count(item, "\n") + 1
		if linesUsed+itemLines > availableHeight {
			break
		}
		b.WriteString(item)
		b.WriteString("\n")
		linesUsed += itemLines
		visibleCount++
	}

	// Fill remaining space before search bar
	for i := linesUsed; i < availableHeight; i++ {
		b.WriteString("\n")
	}

	// Search bar at bottom
	searchBar := m.renderSearchBar()
	b.WriteString(searchBar)

	// Apply panel style
	style := m.styles.Panel
	if m.focused {
		style = m.styles.PanelActive
	}

	return style.Width(m.width).Height(m.height).Render(b.String())
}

// renderSearchBar renders the search bar at the bottom of the panel
func (m ContactsModel) renderSearchBar() string {
	maxWidth := m.width - 4

	if m.searchMode {
		// Active search mode - show input with cursor
		prompt := "/"
		query := m.searchQuery
		if len(query) > maxWidth-3 {
			query = query[len(query)-(maxWidth-3):]
		}
		cursor := "█"
		searchLine := prompt + query + cursor

		// Show match count
		matches := len(m.getFilteredConversations())
		matchInfo := fmt.Sprintf(" (%d)", matches)
		if len(searchLine)+len(matchInfo) <= maxWidth {
			searchLine += m.styles.ContactTime.Render(matchInfo)
		}

		return m.styles.InputFocused.Width(maxWidth).Render(searchLine)
	}

	// Inactive - show hint
	hint := "/ to search"
	return m.styles.ContactPreview.Render(hint)
}

// renderConversationItem renders a single conversation item
func (m ContactsModel) renderConversationItem(conv *store.Conversation, selected bool) string {
	maxWidth := m.width - 6 // Account for padding, borders, and indicator

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = "> "
	}

	// Format name
	name := conv.Name
	if name == "" {
		name = "Unknown"
	}

	// Unread indicator
	unreadMark := ""
	if conv.Unread {
		unreadMark = "● "
		name = unreadMark + name
	}

	if len(name) > maxWidth-8 {
		name = name[:maxWidth-11] + "..."
	}

	// Format time
	timeStr := formatRelativeTime(conv.LatestTimestamp)

	// Format preview
	preview := conv.LatestMessage
	if preview == "" {
		preview = "(no messages)"
	}
	// Remove newlines from preview
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", "")
	if len(preview) > maxWidth-2 {
		preview = preview[:maxWidth-5] + "..."
	}

	// Build the item
	var itemStyle lipgloss.Style
	if selected {
		itemStyle = m.styles.ContactItemSelected
	} else {
		itemStyle = m.styles.ContactItem
	}

	// First line: indicator + name and time
	nameStyle := m.styles.ContactName
	if conv.Unread {
		nameStyle = m.styles.ContactUnread
	}

	// Calculate spacing between name and time
	spacing := maxWidth - len(name) - len(timeStr)
	if spacing < 1 {
		spacing = 1
	}

	firstLine := indicator + nameStyle.Render(name) + strings.Repeat(" ", spacing) + m.styles.ContactTime.Render(timeStr)

	// Second line: preview (indented to align with name)
	secondLine := "  " + m.styles.ContactPreview.Render(preview)

	return itemStyle.Width(m.width - 2).Render(firstLine + "\n" + secondLine)
}

// getFilteredConversations returns conversations filtered by search query using fuzzy matching
func (m ContactsModel) getFilteredConversations() []*store.Conversation {
	if m.searchQuery == "" {
		return m.conversations
	}

	query := strings.ToLower(m.searchQuery)

	// Score and filter conversations
	type scored struct {
		conv  *store.Conversation
		score int
	}

	var results []scored
	for _, conv := range m.conversations {
		name := strings.ToLower(conv.Name)
		score := fuzzyMatch(query, name)
		if score > 0 {
			results = append(results, scored{conv: conv, score: score})
		}
	}

	// Sort by score (highest first)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Extract sorted conversations
	filtered := make([]*store.Conversation, len(results))
	for i, r := range results {
		filtered[i] = r.conv
	}
	return filtered
}

// fuzzyMatch returns a score for how well the query matches the target
// Higher scores are better matches, 0 means no match
func fuzzyMatch(query, target string) int {
	if query == "" {
		return 1
	}
	if target == "" {
		return 0
	}

	// Exact match gets highest score
	if strings.Contains(target, query) {
		// Bonus for match at start
		if strings.HasPrefix(target, query) {
			return 1000 + len(query)
		}
		return 500 + len(query)
	}

	// Fuzzy match - all query chars must appear in order
	queryIdx := 0
	score := 0
	lastMatchIdx := -1
	consecutive := 0

	for i := 0; i < len(target) && queryIdx < len(query); i++ {
		if target[i] == query[queryIdx] {
			score += 10

			// Bonus for consecutive matches
			if lastMatchIdx == i-1 {
				consecutive++
				score += consecutive * 5
			} else {
				consecutive = 0
			}

			// Bonus for match at word boundary
			if i == 0 || target[i-1] == ' ' || target[i-1] == '-' || target[i-1] == '_' {
				score += 20
			}

			lastMatchIdx = i
			queryIdx++
		}
	}

	// All query characters must be found
	if queryIdx < len(query) {
		return 0
	}

	return score
}

// visibleItemCount returns the number of items that can be displayed
func (m ContactsModel) visibleItemCount() int {
	return (m.height - 3) / 2 // Each item takes 2 lines
}

// SetConversations updates the conversation list
func (m *ContactsModel) SetConversations(convs []*store.Conversation) {
	m.conversations = convs
	if m.selected >= len(convs) {
		m.selected = max(0, len(convs)-1)
	}
}

// SetSize sets the panel dimensions
func (m *ContactsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets the focus state
func (m *ContactsModel) SetFocused(focused bool) {
	m.focused = focused
}

// SelectedConversation returns the currently selected conversation
func (m ContactsModel) SelectedConversation() *store.Conversation {
	convs := m.getFilteredConversations()
	if m.selected >= 0 && m.selected < len(convs) {
		return convs[m.selected]
	}
	return nil
}

// formatRelativeTime formats a time as a relative string
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	default:
		return t.Format("Jan 2")
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
