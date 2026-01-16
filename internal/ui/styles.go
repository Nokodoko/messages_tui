package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors used throughout the application
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#7C3AED") // Purple
	SecondaryColor = lipgloss.Color("#10B981") // Green
	AccentColor    = lipgloss.Color("#3B82F6") // Blue
	CyanColor      = lipgloss.Color("#06B6D4") // Cyan - for insert mode

	// Neutral colors
	BackgroundColor = lipgloss.Color("#1F2937")
	SurfaceColor    = lipgloss.Color("#374151")
	BorderColor     = lipgloss.Color("#4B5563")

	// Text colors
	TextColor        = lipgloss.Color("#F9FAFB")
	TextMutedColor   = lipgloss.Color("#9CA3AF")
	TextSuccessColor = lipgloss.Color("#10B981")
	TextErrorColor   = lipgloss.Color("#EF4444")
	TextWarningColor = lipgloss.Color("#F59E0B")

	// Message colors
	SentMessageColor     = lipgloss.Color("#7C3AED")
	ReceivedMessageColor = lipgloss.Color("#374151")
)

// Styles for different UI components
type Styles struct {
	// App-level styles
	App             lipgloss.Style
	StatusBar       lipgloss.Style
	StatusBarLeader lipgloss.Style // Special style when leader key is active
	HelpBar         lipgloss.Style

	// Panel styles
	Panel          lipgloss.Style
	PanelActive    lipgloss.Style
	PanelTitle     lipgloss.Style
	PanelTitleText lipgloss.Style

	// Contact list styles
	ContactItem         lipgloss.Style
	ContactItemSelected lipgloss.Style
	ContactName         lipgloss.Style
	ContactPreview      lipgloss.Style
	ContactTime         lipgloss.Style
	ContactUnread       lipgloss.Style

	// Message styles
	MessageSent       lipgloss.Style
	MessageReceived   lipgloss.Style
	MessageTime       lipgloss.Style
	MessageSender     lipgloss.Style
	MessageStatus     lipgloss.Style
	MessageStatusRead lipgloss.Style

	// Input styles
	Input         lipgloss.Style
	InputFocused  lipgloss.Style
	InputPrompt   lipgloss.Style
	InputPlaceholder lipgloss.Style

	// QR code styles
	QRContainer lipgloss.Style
	QRTitle     lipgloss.Style
	QRHelp      lipgloss.Style

	// Dialog styles
	Dialog       lipgloss.Style
	DialogTitle  lipgloss.Style
	DialogButton lipgloss.Style
}

// DefaultStyles returns the default application styles
func DefaultStyles() *Styles {
	s := &Styles{}

	// App-level styles
	s.App = lipgloss.NewStyle().
		Background(BackgroundColor)

	s.StatusBar = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(SurfaceColor).
		Padding(0, 1)

	s.StatusBarLeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(TextWarningColor).
		Bold(true).
		Padding(0, 1)

	s.HelpBar = lipgloss.NewStyle().
		Foreground(TextMutedColor).
		Background(SurfaceColor).
		Padding(0, 1)

	// Panel styles
	s.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1)

	s.PanelActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1)

	s.PanelTitle = lipgloss.NewStyle().
		Background(SurfaceColor).
		Padding(0, 1).
		MarginBottom(1)

	s.PanelTitleText = lipgloss.NewStyle().
		Foreground(TextColor).
		Bold(true)

	// Contact list styles
	s.ContactItem = lipgloss.NewStyle().
		Padding(0, 1).
		MarginBottom(0)

	s.ContactItemSelected = lipgloss.NewStyle().
		Padding(0, 1).
		Background(SurfaceColor).
		Foreground(TextColor)

	s.ContactName = lipgloss.NewStyle().
		Foreground(TextColor).
		Bold(true)

	s.ContactPreview = lipgloss.NewStyle().
		Foreground(TextMutedColor).
		MaxWidth(30)

	s.ContactTime = lipgloss.NewStyle().
		Foreground(TextMutedColor).
		Align(lipgloss.Right)

	s.ContactUnread = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	// Message styles
	s.MessageSent = lipgloss.NewStyle().
		Background(SentMessageColor).
		Foreground(TextColor).
		Padding(0, 1).
		MarginLeft(4).
		Align(lipgloss.Right)

	s.MessageReceived = lipgloss.NewStyle().
		Background(ReceivedMessageColor).
		Foreground(TextColor).
		Padding(0, 1).
		MarginRight(4)

	s.MessageTime = lipgloss.NewStyle().
		Foreground(TextMutedColor).
		Italic(true)

	s.MessageSender = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	s.MessageStatus = lipgloss.NewStyle().
		Foreground(TextMutedColor)

	s.MessageStatusRead = lipgloss.NewStyle().
		Foreground(TextSuccessColor)

	// Input styles
	s.Input = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1)

	s.InputFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1)

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(PrimaryColor)

	s.InputPlaceholder = lipgloss.NewStyle().
		Foreground(TextMutedColor)

	// QR code styles
	s.QRContainer = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Align(lipgloss.Center)

	s.QRTitle = lipgloss.NewStyle().
		Foreground(TextColor).
		Bold(true).
		MarginBottom(1).
		Align(lipgloss.Center)

	s.QRHelp = lipgloss.NewStyle().
		Foreground(TextMutedColor).
		MarginTop(1).
		Align(lipgloss.Center)

	// Dialog styles
	s.Dialog = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Background(SurfaceColor)

	s.DialogTitle = lipgloss.NewStyle().
		Foreground(TextColor).
		Bold(true).
		MarginBottom(1)

	s.DialogButton = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(PrimaryColor).
		Padding(0, 2)

	return s
}

// Truncate truncates a string to the specified width
func Truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}
