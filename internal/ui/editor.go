package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/n0ko/messages-tui/internal/config"
)

// EditorResultMsg is sent when the external editor completes
type EditorResultMsg struct {
	Content string
	Err     error
}

// EditorCancelledMsg is sent when the editor is cancelled (empty content)
type EditorCancelledMsg struct{}

// OpenEditorWithContentMsg is sent when the user wants to open the editor with existing content
type OpenEditorWithContentMsg struct {
	InitialContent string
}

// OpenExternalEditor opens the configured external editor with a temp file
// and returns the content when the editor closes
func OpenExternalEditor(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "messages-tui-compose-*.txt")
		if err != nil {
			return EditorResultMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		// Clean up the temp file when done
		defer os.Remove(tmpPath)

		// Build the editor command
		editor := cfg.Editor
		if editor == "" {
			editor = os.Getenv("EDITOR")
			if editor == "" {
				editor = "nvim"
			}
		}

		// Prepare arguments
		args := append(cfg.EditorArgs, tmpPath)

		// Create the command
		cmd := exec.Command(editor, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Run the editor
		if err := cmd.Run(); err != nil {
			return EditorResultMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read the content
		content, err := os.ReadFile(tmpPath)
		if err != nil {
			return EditorResultMsg{Err: fmt.Errorf("failed to read temp file: %w", err)}
		}

		// Trim whitespace
		text := strings.TrimSpace(string(content))

		// If empty, treat as cancelled
		if text == "" {
			return EditorCancelledMsg{}
		}

		return EditorResultMsg{Content: text}
	}
}

// EditorSession manages the editor subprocess
type EditorSession struct {
	cfg     *config.Config
	tmpPath string
}

// NewEditorSession creates a new editor session with optional initial content
func NewEditorSession(cfg *config.Config, initialContent string) (*EditorSession, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "messages-tui-compose-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write initial content if provided
	if initialContent != "" {
		if _, err := tmpFile.WriteString(initialContent); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("failed to write initial content: %w", err)
		}
	}
	tmpFile.Close()

	return &EditorSession{
		cfg:     cfg,
		tmpPath: tmpPath,
	}, nil
}

// Command returns the exec.Cmd to run the editor
func (e *EditorSession) Command() *exec.Cmd {
	editor := e.cfg.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "nvim"
		}
	}

	args := append(e.cfg.EditorArgs, e.tmpPath)
	return exec.Command(editor, args...)
}

// ReadContent reads the content from the temp file
func (e *EditorSession) ReadContent() (string, error) {
	content, err := os.ReadFile(e.tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

// Cleanup removes the temp file
func (e *EditorSession) Cleanup() {
	os.Remove(e.tmpPath)
}

// StartEditorCmd starts the editor and returns the result
// This properly suspends the TUI while the editor is running
func StartEditorCmd(cfg *config.Config, initialContent string) tea.Cmd {
	session, err := NewEditorSession(cfg, initialContent)
	if err != nil {
		return func() tea.Msg {
			return EditorResultMsg{Err: err}
		}
	}

	cmd := session.Command()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer session.Cleanup()

		if err != nil {
			return EditorResultMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		content, err := session.ReadContent()
		if err != nil {
			return EditorResultMsg{Err: err}
		}

		if content == "" {
			return EditorCancelledMsg{}
		}

		return EditorResultMsg{Content: content}
	})
}
