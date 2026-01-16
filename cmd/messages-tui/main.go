package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/n0ko/messages-tui/internal/client"
	"github.com/n0ko/messages-tui/internal/config"
	"github.com/n0ko/messages-tui/internal/store"
	"github.com/n0ko/messages-tui/internal/ui"
)

var version = "dev"

func main() {
	// Define flags
	clearSession := flag.Bool("clear-session", false, "Clear saved session and re-pair with phone")
	showVersion := flag.Bool("version", false, "Show version information")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `messages-tui - Terminal UI for Google Messages

Usage:
  messages-tui [flags]

Flags:
  -clear-session    Clear saved session and re-pair with phone
  -version          Show version information
  -h, -help         Show this help message

Key Bindings:
  j/k or ↑/↓        Navigate messages/contacts
  Tab               Switch panels
  Shift+Tab         Switch panels (reverse)
  Enter             Select conversation / Send message
  e or Ctrl+E       Compose in external editor
  /                 Search conversations
  q or Ctrl+C       Quit

File Locations:
  Config:   ~/.config/messages-tui/config.yaml
  Session:  ~/.config/messages-tui/session.json
  Logs:     ~/.config/messages-tui/messages-tui.log

First Launch:
  1. Run messages-tui
  2. Scan the QR code with Google Messages on your Android phone
     (Menu ⋮ → Device Pairing → QR Scanner)
  3. Your conversations will sync automatically

`)
	}

	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("messages-tui %s\n", version)
		os.Exit(0)
	}

	// Handle clear-session flag
	if *clearSession {
		st := store.New()
		if err := st.ClearSession(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing session: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Session cleared. Run messages-tui to pair again.")
		os.Exit(0)
	}

	// Set up logging
	logFile, err := setupLogging()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not set up logging: %v\n", err)
	} else {
		defer logFile.Close()
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create store
	st := store.New()

	// Create client
	cl := client.New(st)

	// Create application
	app := ui.NewApp(cfg, st, cl)

	// Set up context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Try to restore session or start pairing
	go func() {
		if err := initializeClient(ctx, st, cl, app); err != nil {
			app.SetError(err)
		}
	}()

	// Create and run the Bubble Tea program
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

// setupLogging sets up logging to a file
func setupLogging() (*os.File, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	logPath := dir + "/messages-tui.log"
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return f, nil
}

// initializeClient initializes the client connection
func initializeClient(ctx context.Context, st *store.Store, cl *client.Client, app *ui.App) error {
	// Create auth handler
	auth := client.NewAuthHandler(st)
	defer auth.Close()

	// Try to restore existing session
	gmClient, err := auth.RestoreSession(ctx)
	if err != nil {
		log.Printf("Failed to restore session: %v", err)
	}

	if gmClient != nil {
		// Session restored successfully
		log.Println("Session restored")
		cl.SetClient(gmClient)
		app.SetConnected()
		return nil
	}

	// Need to pair via QR code
	log.Println("Starting QR pairing...")
	gmClient, err = auth.StartPairing(ctx)
	if err != nil {
		return fmt.Errorf("failed to start pairing: %w", err)
	}

	// Wait for QR code or completion
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case qr := <-auth.QRChannel():
			log.Println("QR code received")
			app.SetQRCode(qr.URL)

		case err := <-auth.ErrorChannel():
			return err

		case <-auth.DoneChannel():
			log.Println("Pairing completed, connecting client...")
			// After pairing, we need to explicitly connect for messaging
			if err := gmClient.Connect(); err != nil {
				return fmt.Errorf("failed to connect after pairing: %w", err)
			}
			log.Println("Client connected successfully")
			cl.SetClient(gmClient)
			app.SetConnected()
			return nil
		}
	}
}
