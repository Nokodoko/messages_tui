package ui

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skip2/go-qrcode"

	"github.com/n0ko/messages-tui/internal/client"
	"github.com/n0ko/messages-tui/internal/config"
	"github.com/n0ko/messages-tui/internal/store"
)

// AppState represents the current state of the application
type AppState int

const (
	StateLoading AppState = iota
	StateQRPairing
	StateConnected
	StateError
)

// FocusedPanel represents which panel is currently focused
type FocusedPanel int

const (
	PanelContacts FocusedPanel = iota
	PanelMessages
	PanelInput
)

// AppKeyMap defines the global key bindings
type AppKeyMap struct {
	Quit      key.Binding
	Tab       key.Binding
	ShiftTab  key.Binding
	Help      key.Binding
	Refresh   key.Binding
}

// DefaultAppKeyMap returns the default global key bindings
func DefaultAppKeyMap() AppKeyMap {
	return AppKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh"),
		),
	}
}

// App is the main application model
type App struct {
	// Configuration
	cfg    *config.Config
	styles *Styles
	keyMap AppKeyMap

	// State
	state            AppState
	focusedPanel     FocusedPanel
	err              error
	statusMsg        string
	leaderKeyPressed bool // Track if leader key was pressed (for leader+key combos)

	// Size
	width  int
	height int

	// Components
	contacts ContactsModel
	messages MessagesModel
	input    InputModel

	// Active conversation for messaging (set by pressing Enter in contacts)
	activeConversationID string

	// QR pairing
	qrURL string

	// External message channel for receiving messages from outside Bubble Tea loop
	externalMsgs chan tea.Msg

	// Backend
	client *client.Client
	store  *store.Store

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, st *store.Store, cl *client.Client) *App {
	ctx, cancel := context.WithCancel(context.Background())
	styles := DefaultStyles()

	return &App{
		cfg:          cfg,
		styles:       styles,
		keyMap:       KeyMapFromConfig(cfg),
		state:        StateLoading,
		contacts:     NewContactsModel(styles),
		messages:     NewMessagesModel(styles),
		input:        NewInputModel(styles),
		client:       cl,
		store:        st,
		ctx:          ctx,
		cancel:       cancel,
		externalMsgs: make(chan tea.Msg, 10),
	}
}

// KeyMapFromConfig builds an AppKeyMap from the configuration
func KeyMapFromConfig(cfg *config.Config) AppKeyMap {
	kb := cfg.Keybinds
	return AppKeyMap{
		Quit: key.NewBinding(
			key.WithKeys(kb.Global.Quit, "ctrl+c"),
			key.WithHelp(kb.Global.Quit, "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys(kb.Global.NextPanel),
			key.WithHelp(kb.Global.NextPanel, "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys(kb.Global.PrevPanel),
			key.WithHelp(kb.Global.PrevPanel, "prev panel"),
		),
		Help: key.NewBinding(
			key.WithKeys(kb.Global.Help),
			key.WithHelp(kb.Global.Help, "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys(kb.Global.Refresh),
			key.WithHelp(kb.Global.Refresh, "refresh"),
		),
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.input.Init(),
		a.listenForEvents(),
		a.listenForExternalMsgs(),
	)
}

// listenForExternalMsgs listens for messages from outside the Bubble Tea loop
func (a *App) listenForExternalMsgs() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-a.ctx.Done():
			return nil
		case msg := <-a.externalMsgs:
			return msg
		}
	}
}

// Update handles all application messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateSizes()

	case tea.KeyMsg:
		// Handle leader key combinations first
		if a.leaderKeyPressed {
			// Escape cancels leader mode
			if msg.String() == "esc" {
				a.leaderKeyPressed = false
				a.statusMsg = ""
				return a, nil
			}
			a.leaderKeyPressed = false // Reset leader state
			if cmd := a.handleLeaderKey(msg); cmd != nil {
				return a, cmd
			}
			// If leader combo wasn't handled, check if it was a valid navigation
			if a.handleLeaderNavigation(msg) {
				return a, nil
			}
			// Leader was pressed but next key wasn't a valid combo - continue normal handling
		}

		// Check if leader key was pressed
		if a.matchesLeaderKey(msg) {
			a.leaderKeyPressed = true
			a.statusMsg = "-- LEADER --"
			return a, nil
		}

		// Handle global keys
		switch {
		case key.Matches(msg, a.keyMap.Quit):
			// Don't quit if input is focused and has content
			if a.focusedPanel == PanelInput && a.input.Value() != "" {
				break
			}
			a.cancel()
			return a, tea.Quit

		case key.Matches(msg, a.keyMap.Tab):
			a.cycleFocus(1)
			return a, nil

		case key.Matches(msg, a.keyMap.ShiftTab):
			a.cycleFocus(-1)
			return a, nil
		}

		// Handle state-specific input
		switch a.state {
		case StateConnected:
			cmds = append(cmds, a.handleConnectedInput(msg))
		}

	case client.Event:
		cmds = append(cmds, a.handleClientEvent(msg))

	case SendMessageMsg:
		log.Printf("App: SendMessageMsg received, content length: %d", len(msg.Content))
		cmds = append(cmds, a.sendMessage(msg.Content))

	case OpenEditorMsg:
		return a, StartEditorCmd(a.cfg, msg.InitialContent)

	case EditorResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Editor error: %v", msg.Err)
		} else {
			// Put content in input box for review before sending
			a.input.SetValue(msg.Content)
			// Focus the input panel
			a.contacts.SetFocused(false)
			a.messages.SetFocused(false)
			a.input.SetFocused(true)
			a.focusedPanel = PanelInput
			a.statusMsg = "Press Enter to send"
		}

	case EditorCancelledMsg:
		a.statusMsg = "Message cancelled"

	case conversationsLoadedMsg:
		log.Printf("App: Received conversationsLoadedMsg with %d conversations", len(msg.conversations))
		a.contacts.SetConversations(msg.conversations)
		a.statusMsg = fmt.Sprintf("Loaded %d conversations", len(msg.conversations))

	case messagesLoadedMsg:
		a.messages.SetMessages(msg.conversationID, msg.messages)

	case messageSentMsg:
		log.Printf("App: Message sent, refreshing conversation")
		a.statusMsg = "Message sent"
		// Clear sending indicator
		a.input, _ = a.input.Update(MessageSentNotifyMsg{})
		// Reload active conversation to show the sent message
		if a.activeConversationID != "" {
			cmds = append(cmds, a.loadMessages(a.activeConversationID))
		}

	case qrCodeMsg:
		log.Printf("App: Received qrCodeMsg, transitioning to QRPairing state")
		a.state = StateQRPairing
		a.qrURL = msg.url
		// Continue listening for more external messages
		cmds = append(cmds, a.listenForExternalMsgs())

	case connectedMsg:
		log.Printf("App: Received connectedMsg, transitioning to Connected state")
		a.state = StateConnected
		a.statusMsg = "Connected"
		a.contacts.SetFocused(true)
		cmds = append(cmds, a.loadConversations())
		// Continue listening for more external messages
		cmds = append(cmds, a.listenForExternalMsgs())

	case errorMsg:
		log.Printf("App: Received errorMsg: %v", msg.err)
		a.err = msg.err
		a.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		// Clear sending indicator on error
		a.input, _ = a.input.Update(MessageFailedNotifyMsg{})
		// Only transition to error state if not already connected
		if a.state != StateConnected {
			a.state = StateError
		}
		// Continue listening for more external messages
		cmds = append(cmds, a.listenForExternalMsgs())
	}

	// Update focused component
	switch a.focusedPanel {
	case PanelContacts:
		var cmd tea.Cmd
		a.contacts, cmd = a.contacts.Update(msg)
		cmds = append(cmds, cmd)

		// Check if selection changed
		if conv := a.contacts.SelectedConversation(); conv != nil {
			cmds = append(cmds, a.loadMessages(conv.ID))
		}

	case PanelMessages:
		var cmd tea.Cmd
		a.messages, cmd = a.messages.Update(msg)
		cmds = append(cmds, cmd)

	case PanelInput:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View renders the application
func (a *App) View() string {
	switch a.state {
	case StateLoading:
		return a.renderLoading()
	case StateQRPairing:
		return a.renderQRPairing()
	case StateError:
		return a.renderError()
	case StateConnected:
		return a.renderConnected()
	default:
		return "Unknown state"
	}
}

// renderLoading renders the loading screen
func (a *App) renderLoading() string {
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		a.styles.QRTitle.Render("Connecting to Google Messages..."),
	)
}

// renderQRPairing renders the QR code pairing screen
func (a *App) renderQRPairing() string {
	var content strings.Builder

	content.WriteString(a.styles.QRTitle.Render("Scan QR with Google Messages"))
	content.WriteString("\n\n")

	// Generate QR code at render time with appropriate size for terminal
	if a.qrURL != "" {
		qr, err := qrcode.New(a.qrURL, qrcode.Medium)
		if err == nil {
			// ToSmallString uses 2 characters per module horizontally
			// Calculate max QR size that fits in available space
			// Reserve space for border, padding, and help text
			availableWidth := a.width - 10  // borders and padding
			availableHeight := a.height - 12 // title, help text, borders

			// QR code modules: each row is 1 line, each column is 2 chars
			// Standard QR for this data is about 25-29 modules
			// ToSmallString produces roughly 2*modules + 2 chars wide
			qrStr := qr.ToSmallString(false)
			qrLines := strings.Split(qrStr, "\n")

			// Check if QR fits, if not we can't do much but show it anyway
			qrHeight := len(qrLines)
			qrWidth := 0
			if len(qrLines) > 0 {
				qrWidth = len(qrLines[0])
			}

			// If QR is too large for terminal, show a warning
			if qrWidth > availableWidth || qrHeight > availableHeight {
				content.WriteString(a.styles.QRHelp.Render("(Resize terminal for better view)"))
				content.WriteString("\n")
			}

			content.WriteString(qrStr)
		} else {
			content.WriteString("Failed to generate QR code")
		}
	} else {
		content.WriteString("Waiting for QR code...")
	}

	content.WriteString("\n")
	content.WriteString(a.styles.QRHelp.Render("Open Google Messages on your phone"))
	content.WriteString("\n")
	content.WriteString(a.styles.QRHelp.Render("Tap ⋮ → Device Pairing → QR Scanner"))

	box := a.styles.QRContainer.Render(content.String())

	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

// renderError renders the error screen
func (a *App) renderError() string {
	errMsg := "An error occurred"
	if a.err != nil {
		errMsg = a.err.Error()
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		a.styles.DialogTitle.Render("Error"),
		"",
		a.styles.ContactPreview.Render(errMsg),
		"",
		a.styles.QRHelp.Render("Press q to quit"),
	)

	box := a.styles.Dialog.Render(content)

	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

// renderConnected renders the main connected view
func (a *App) renderConnected() string {
	// Status bar at top (1 line)
	statusBar := a.renderStatusBar()

	// Help bar at bottom (1 line)
	helpBar := a.renderHelpBar()

	// Main content area height (total - status bar - help bar)
	// Account for panel borders (2 lines each for top/bottom)
	contentHeight := a.height - 2

	// Panel widths - account for borders (2 chars each panel = 4 total)
	// Each panel has left and right borders drawn outside the content width
	contactsWidth := a.width / 4
	if contactsWidth < 20 {
		contactsWidth = 20
	}
	// Subtract 4 for borders: 2 for contacts panel borders + 2 for messages/input panel borders
	messagesWidth := a.width - contactsWidth - 4
	if messagesWidth < 20 {
		messagesWidth = 20
	}
	// Ensure total doesn't exceed terminal width
	if contactsWidth+messagesWidth+4 > a.width {
		// Reduce contacts width to fit
		contactsWidth = a.width - messagesWidth - 4
		if contactsWidth < 15 {
			contactsWidth = 15
			messagesWidth = a.width - contactsWidth - 4
		}
	}

	// Input takes 3 lines (border + content + border)
	inputHeight := 3
	// Messages panel height: content area minus input, minus borders (2 for contacts border overlap)
	messagesHeight := contentHeight - inputHeight - 2

	// Set sizes - subtract border space from heights
	a.contacts.SetSize(contactsWidth, contentHeight-2)
	a.messages.SetSize(messagesWidth, messagesHeight)
	a.input.SetWidth(messagesWidth)

	// Render panels
	contactsView := a.contacts.View()
	messagesView := a.messages.View()
	inputView := a.input.View()

	// Combine messages and input
	rightPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		messagesView,
		inputView,
	)

	// Combine contacts and right panel
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		contactsView,
		rightPanel,
	)

	// Final layout with fixed height - use Place to constrain to exact terminal size
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		statusBar,
		mainContent,
		helpBar,
	)

	// Constrain to terminal dimensions
	return lipgloss.NewStyle().
		MaxHeight(a.height).
		MaxWidth(a.width).
		Render(content)
}

// renderStatusBar renders the status bar
func (a *App) renderStatusBar() string {
	// Left side: app name and status
	left := "Messages TUI"
	if a.statusMsg != "" {
		left = a.statusMsg
	}

	// Right side: focused panel indicator
	var panelName string
	switch a.focusedPanel {
	case PanelContacts:
		panelName = "[Contacts]"
	case PanelMessages:
		panelName = "[Messages]"
	case PanelInput:
		panelName = "[Input]"
	}

	// Calculate spacing
	spacing := a.width - len(left) - len(panelName) - 2
	if spacing < 1 {
		spacing = 1
	}

	status := left + strings.Repeat(" ", spacing) + panelName

	// Use special style when leader key is active
	if a.leaderKeyPressed {
		return a.styles.StatusBarLeader.Width(a.width).Render(status)
	}
	return a.styles.StatusBar.Width(a.width).Render(status)
}

// renderHelpBar renders the help bar
func (a *App) renderHelpBar() string {
	kb := a.cfg.Keybinds

	// Show leader key options when leader is pressed
	if a.leaderKeyPressed {
		help := fmt.Sprintf("%s: conversations | %s: messages | %s: input | r: refresh | q: quit | Esc: cancel",
			kb.Navigation.Conversations, kb.Navigation.Messages, kb.Navigation.Input)
		return a.styles.HelpBar.Width(a.width).Render(help)
	}

	leaderHint := fmt.Sprintf("%s+%s/%s/%s: panels",
		kb.LeaderKey, kb.Navigation.Conversations, kb.Navigation.Messages, kb.Navigation.Input)

	var help string
	switch a.focusedPanel {
	case PanelContacts:
		help = fmt.Sprintf("↑/k ↓/j: navigate | Enter: select | /: search | %s | q: quit", leaderHint)
	case PanelMessages:
		help = fmt.Sprintf("↑/k ↓/j: scroll | %s | q: quit", leaderHint)
	case PanelInput:
		if a.input.Mode() == ModeNormal {
			help = fmt.Sprintf("[NORMAL] i: insert | v: editor | d: clear | Enter: send | %s", leaderHint)
		} else {
			help = fmt.Sprintf("[INSERT] Esc: normal mode | Enter: send | %s", leaderHint)
		}
	default:
		help = fmt.Sprintf("Tab: switch panel | %s | q: quit", leaderHint)
	}
	return a.styles.HelpBar.Width(a.width).Render(help)
}

// updateSizes updates component sizes based on current terminal dimensions
func (a *App) updateSizes() {
	// Same calculations as renderConnected for consistency
	contentHeight := a.height - 2

	contactsWidth := a.width / 4
	if contactsWidth < 20 {
		contactsWidth = 20
	}
	messagesWidth := a.width - contactsWidth - 4
	if messagesWidth < 20 {
		messagesWidth = 20
	}
	if contactsWidth+messagesWidth+4 > a.width {
		contactsWidth = a.width - messagesWidth - 4
		if contactsWidth < 15 {
			contactsWidth = 15
			messagesWidth = a.width - contactsWidth - 4
		}
	}

	inputHeight := 3
	messagesHeight := contentHeight - inputHeight - 2

	a.contacts.SetSize(contactsWidth, contentHeight-2)
	a.messages.SetSize(messagesWidth, messagesHeight)
	a.input.SetWidth(messagesWidth)
}

// cycleFocus cycles through the panels
func (a *App) cycleFocus(direction int) {
	// Update focus states
	a.contacts.SetFocused(false)
	a.messages.SetFocused(false)
	a.input.SetFocused(false)

	// Cycle
	numPanels := 3
	a.focusedPanel = FocusedPanel((int(a.focusedPanel) + direction + numPanels) % numPanels)

	// Set new focus
	switch a.focusedPanel {
	case PanelContacts:
		a.contacts.SetFocused(true)
	case PanelMessages:
		a.messages.SetFocused(true)
	case PanelInput:
		a.input.SetFocused(true)
	}
}

// focusPanel focuses a specific panel directly
func (a *App) focusPanel(panel FocusedPanel) {
	// Update focus states
	a.contacts.SetFocused(false)
	a.messages.SetFocused(false)
	a.input.SetFocused(false)

	a.focusedPanel = panel

	// Set new focus
	switch panel {
	case PanelContacts:
		a.contacts.SetFocused(true)
		a.statusMsg = "Conversations"
	case PanelMessages:
		a.messages.SetFocused(true)
		a.statusMsg = "Messages"
	case PanelInput:
		a.input.SetFocused(true)
		a.statusMsg = "Input"
	}
}

// handleLeaderKey handles commands after leader key is pressed
// Returns a command if handled, nil otherwise
func (a *App) handleLeaderKey(msg tea.KeyMsg) tea.Cmd {
	// Currently no leader+key commands that return a command
	// This is a placeholder for future extensions like leader+r for refresh
	kb := a.cfg.Keybinds
	keyStr := msg.String()

	// leader+r for refresh
	if keyStr == "r" {
		a.statusMsg = "Refreshing..."
		return a.loadConversations()
	}

	// Check for quit with leader
	if keyStr == kb.Global.Quit {
		a.cancel()
		return tea.Quit
	}

	return nil
}

// matchesLeaderKey checks if the key message matches the configured leader key
// Handles various representations of key combinations like ctrl+space
func (a *App) matchesLeaderKey(msg tea.KeyMsg) bool {
	leaderKey := a.cfg.Keybinds.LeaderKey
	keyStr := msg.String()

	// Direct match
	if keyStr == leaderKey {
		return true
	}

	// Handle ctrl+space specifically - bubbletea represents it as "ctrl+ " or NUL
	if leaderKey == "ctrl+space" {
		// ctrl+space can be: "ctrl+ " (with space), ctrl+@ (NUL), or the actual space with ctrl
		if keyStr == "ctrl+ " || keyStr == "ctrl+@" || (msg.Type == tea.KeyCtrlAt) {
			return true
		}
		// Also check for raw NUL character (ASCII 0)
		if len(keyStr) == 1 && keyStr[0] == 0 {
			return true
		}
	}

	// Handle other ctrl+key combinations
	// Convert "ctrl+x" format to check against msg
	if len(leaderKey) > 5 && leaderKey[:5] == "ctrl+" {
		expectedKey := leaderKey[5:]
		if msg.Alt == false && keyStr == "ctrl+"+expectedKey {
			return true
		}
	}

	return false
}

// handleLeaderNavigation handles panel navigation after leader key
// Returns true if navigation was handled
func (a *App) handleLeaderNavigation(msg tea.KeyMsg) bool {
	kb := a.cfg.Keybinds
	keyStr := msg.String()

	switch keyStr {
	case kb.Navigation.Conversations:
		a.focusPanel(PanelContacts)
		return true
	case kb.Navigation.Messages:
		a.focusPanel(PanelMessages)
		return true
	case kb.Navigation.Input:
		a.focusPanel(PanelInput)
		return true
	}

	return false
}

// handleConnectedInput handles input when connected
func (a *App) handleConnectedInput(msg tea.KeyMsg) tea.Cmd {
	// Handle Enter on contacts to select conversation and load messages
	if a.focusedPanel == PanelContacts && msg.String() == "enter" {
		if conv := a.contacts.SelectedConversation(); conv != nil {
			a.activeConversationID = conv.ID
			a.statusMsg = fmt.Sprintf("Selected: %s", conv.Name)
			return a.loadMessages(conv.ID)
		}
	}
	return nil
}

// handleClientEvent handles events from the client
func (a *App) handleClientEvent(evt client.Event) tea.Cmd {
	switch evt.Type {
	case client.EventTypeConnected:
		return func() tea.Msg { return connectedMsg{} }

	case client.EventTypeDisconnected:
		a.statusMsg = "Disconnected"

	case client.EventTypeNewMessage:
		if evt.Message != nil {
			a.messages.AddMessage(evt.Message)
			// Update conversation list
			return a.loadConversations()
		}

	case client.EventTypeConversationsUpdated:
		return a.loadConversations()

	case client.EventTypeError:
		if evt.Error != nil {
			a.statusMsg = fmt.Sprintf("Error: %v", evt.Error)
		}
	}
	return nil
}

// listenForEvents starts listening for client events
func (a *App) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case <-a.ctx.Done():
				return nil
			case evt, ok := <-a.client.EventChannel():
				if !ok {
					return nil
				}
				return evt
			}
		}
	}
}

// loadConversations loads conversations from the client
func (a *App) loadConversations() tea.Cmd {
	return func() tea.Msg {
		log.Printf("App: Loading conversations...")
		convs, err := a.client.ListConversations(a.ctx)
		if err != nil {
			log.Printf("App: Failed to load conversations: %v", err)
			return errorMsg{err: err}
		}
		log.Printf("App: Loaded %d conversations", len(convs))
		return conversationsLoadedMsg{conversations: convs}
	}
}

// loadMessages loads messages for a conversation
func (a *App) loadMessages(conversationID string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := a.client.GetMessages(a.ctx, conversationID)
		if err != nil {
			return errorMsg{err: err}
		}
		return messagesLoadedMsg{
			conversationID: conversationID,
			messages:       msgs,
		}
	}
}

// sendMessage sends a message to the active conversation
func (a *App) sendMessage(content string) tea.Cmd {
	if a.activeConversationID == "" {
		log.Printf("App: sendMessage - no conversation selected")
		a.statusMsg = "Select a conversation first! (Enter in contacts)"
		// Clear sending indicator
		a.input, _ = a.input.Update(MessageFailedNotifyMsg{})
		return nil
	}

	log.Printf("App: Sending message to conversation %s", a.activeConversationID)
	convID := a.activeConversationID
	return func() tea.Msg {
		err := a.client.SendMessage(a.ctx, convID, content)
		if err != nil {
			log.Printf("App: SendMessage error: %v", err)
			return errorMsg{err: err}
		}
		log.Printf("App: Message sent successfully")
		return messageSentMsg{}
	}
}

// Message types for internal communication
type conversationsLoadedMsg struct {
	conversations []*store.Conversation
}

type messagesLoadedMsg struct {
	conversationID string
	messages       []*store.Message
}

type qrCodeMsg struct {
	url string
}

type connectedMsg struct{}

type errorMsg struct {
	err error
}

type messageSentMsg struct{}

// SetQRCode sends a QR code URL to the app through the message channel
func (a *App) SetQRCode(url string) {
	a.externalMsgs <- qrCodeMsg{url: url}
}

// SetConnected sends a connected message to the app through the message channel
func (a *App) SetConnected() {
	a.externalMsgs <- connectedMsg{}
}

// SetError sends an error to the app through the message channel
func (a *App) SetError(err error) {
	a.externalMsgs <- errorMsg{err: err}
}
