package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/mautrix-gmessages/pkg/libgm"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/events"

	"github.com/n0ko/messages-tui/internal/store"
)

// QRCodeData contains QR code information for display
type QRCodeData struct {
	// ASCII is the QR code rendered as ASCII art
	ASCII string
	// URL is the raw pairing URL
	URL string
}

// AuthHandler handles the QR code pairing flow
type AuthHandler struct {
	mu      sync.Mutex
	client  *libgm.Client
	store   *store.Store
	qrChan  chan *QRCodeData
	errChan chan error
	done    chan struct{}
	logger  zerolog.Logger

	// Track pairing state - need both for successful connection
	pairSuccessful bool
	clientReady    bool
	doneClosed     bool
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(st *store.Store) *AuthHandler {
	// Create a logger that discards output (we'll use our own logging)
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	return &AuthHandler{
		store:   st,
		qrChan:  make(chan *QRCodeData, 1),
		errChan: make(chan error, 1),
		done:    make(chan struct{}),
		logger:  logger,
	}
}

// QRChannel returns the channel that receives QR codes
func (a *AuthHandler) QRChannel() <-chan *QRCodeData {
	return a.qrChan
}

// ErrorChannel returns the channel that receives errors
func (a *AuthHandler) ErrorChannel() <-chan error {
	return a.errChan
}

// DoneChannel returns the channel that signals completion
func (a *AuthHandler) DoneChannel() <-chan struct{} {
	return a.done
}

// StartPairing initiates the QR code pairing flow
func (a *AuthHandler) StartPairing(ctx context.Context) (*libgm.Client, error) {
	// Create auth data for a new pairing
	authData := libgm.NewAuthData()

	// Create a new client for pairing
	client := libgm.NewClient(authData, nil, a.logger)
	a.client = client

	// Set up event handler for pairing
	client.SetEventHandler(a.handlePairingEvent)

	// Start the login/pairing process
	pairingURL, err := client.StartLogin()
	if err != nil {
		return nil, fmt.Errorf("failed to start login: %w", err)
	}

	// Generate QR code from pairing URL
	qr, err := qrcode.New(pairingURL, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Send the QR code
	ascii := qr.ToSmallString(false)
	a.qrChan <- &QRCodeData{
		ASCII: ascii,
		URL:   pairingURL,
	}

	return client, nil
}

// handlePairingEvent handles events during the pairing process
func (a *AuthHandler) handlePairingEvent(evt any) {
	switch e := evt.(type) {
	case *events.QR:
		log.Printf("Auth: Received QR event")
		// Generate ASCII QR code from the event
		qr, err := qrcode.New(e.URL, qrcode.Medium)
		if err != nil {
			a.errChan <- fmt.Errorf("failed to generate QR code: %w", err)
			return
		}

		ascii := qr.ToSmallString(false)
		a.qrChan <- &QRCodeData{
			ASCII: ascii,
			URL:   e.URL,
		}

	case *events.PairSuccessful:
		log.Printf("Auth: Pairing successful")
		// Save the session
		session := &store.Session{}

		// Marshal the auth data
		if a.client != nil && a.client.AuthData != nil {
			if authData, err := json.Marshal(a.client.AuthData); err == nil {
				session.DevicePair = authData
			}
		}

		if err := a.store.SaveSession(session); err != nil {
			a.errChan <- fmt.Errorf("failed to save session: %w", err)
			return
		}

		a.mu.Lock()
		a.pairSuccessful = true
		a.checkAndSignalDone()
		a.mu.Unlock()

	case *events.ClientReady:
		log.Printf("Auth: Client ready")
		a.mu.Lock()
		a.clientReady = true
		a.checkAndSignalDone()
		a.mu.Unlock()

	case *events.ListenRecovered:
		// ListenRecovered indicates the connection is established after pairing
		log.Printf("Auth: Listen recovered (connection established)")
		a.mu.Lock()
		a.clientReady = true
		a.checkAndSignalDone()
		a.mu.Unlock()

	case *events.ListenFatalError:
		log.Printf("Auth: Fatal error: %v", e.Error)
		a.errChan <- fmt.Errorf("fatal error during pairing: %v", e.Error)

	case *events.ListenTemporaryError:
		log.Printf("Auth: Temporary error: %v", e.Error)

	default:
		log.Printf("Auth: Unknown event type: %T", evt)
	}
}

// checkAndSignalDone signals completion if both pairing and client are ready
// Must be called with mutex held
func (a *AuthHandler) checkAndSignalDone() {
	if a.pairSuccessful && a.clientReady && !a.doneClosed {
		log.Printf("Auth: Both pairing successful and client ready, signaling done")
		a.doneClosed = true
		close(a.done)
	}
}

// RestoreSession attempts to restore a saved session
func (a *AuthHandler) RestoreSession(ctx context.Context) (*libgm.Client, error) {
	session, err := a.store.LoadSession()
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	if session == nil {
		return nil, nil // No session to restore
	}

	// Parse the auth data
	var authData libgm.AuthData
	if err := json.Unmarshal(session.DevicePair, &authData); err != nil {
		// Session is corrupted, clear it
		_ = a.store.ClearSession()
		return nil, nil
	}

	// Create client with saved session
	client := libgm.NewClient(&authData, nil, a.logger)
	a.client = client

	// Try to connect
	if err := client.Connect(); err != nil {
		// Session might be expired, clear it
		_ = a.store.ClearSession()
		return nil, nil
	}

	// Update last used time
	_ = a.store.SaveSession(session)

	return client, nil
}

// Close cleans up the auth handler
func (a *AuthHandler) Close() {
	if a.client != nil {
		a.client.Disconnect()
	}
}
