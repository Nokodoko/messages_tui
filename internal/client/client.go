package client

import (
	"context"
	"fmt"
	"log"
	"sync"

	"go.mau.fi/mautrix-gmessages/pkg/libgm"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/events"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"

	"github.com/n0ko/messages-tui/internal/store"
)

// Client wraps libgm.Client with additional functionality
type Client struct {
	mu        sync.RWMutex
	client    *libgm.Client
	store     *store.Store
	eventChan chan Event
	connected bool
}

// New creates a new Client instance
func New(st *store.Store) *Client {
	return &Client{
		store:     st,
		eventChan: make(chan Event, 100),
	}
}

// EventChannel returns the channel for receiving events
func (c *Client) EventChannel() <-chan Event {
	return c.eventChan
}

// SetClient sets the underlying libgm client
func (c *Client) SetClient(client *libgm.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client = client
	if client != nil {
		client.SetEventHandler(c.handleEvent)
		c.connected = true
	}
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Connect connects to Google Messages
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.connected = true
	c.eventChan <- Event{Type: EventTypeConnected}

	return nil
}

// Disconnect disconnects from Google Messages
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		c.client.Disconnect()
	}
	c.connected = false
	c.eventChan <- Event{Type: EventTypeDisconnected}
}

// handleEvent handles events from libgm
func (c *Client) handleEvent(evt any) {
	switch e := evt.(type) {
	case *events.ListenFatalError:
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		c.eventChan <- Event{
			Type:  EventTypeError,
			Error: fmt.Errorf("fatal error: %v", e.Error),
		}

	case *events.ListenTemporaryError:
		c.eventChan <- Event{
			Type:  EventTypeError,
			Error: fmt.Errorf("temporary error: %v", e.Error),
		}

	case *events.ClientReady:
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
		c.eventChan <- Event{Type: EventTypeConnected}

	case *gmproto.Message:
		msg := convertMessage(e, e.GetConversationID())
		if msg != nil {
			c.store.AddMessage(msg)
			c.eventChan <- Event{
				Type:    EventTypeNewMessage,
				Message: msg,
			}
		}

	case *events.AccountChange:
		// Account state changed, refresh conversations
		c.eventChan <- Event{
			Type: EventTypeConversationsUpdated,
		}
	}
}

// ListConversations fetches the list of conversations
func (c *Client) ListConversations(ctx context.Context) ([]*store.Conversation, error) {
	log.Printf("Client: ListConversations called")
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		log.Printf("Client: client is nil")
		return nil, fmt.Errorf("client not connected")
	}

	log.Printf("Client: Calling libgm ListConversations...")
	resp, err := client.ListConversations(25, gmproto.ListConversationsRequest_UNKNOWN)
	if err != nil {
		log.Printf("Client: ListConversations error: %v", err)
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}

	log.Printf("Client: Got response with %d conversations", len(resp.GetConversations()))
	convs := make([]*store.Conversation, 0, len(resp.GetConversations()))
	for _, conv := range resp.GetConversations() {
		if converted := convertConversation(conv); converted != nil {
			convs = append(convs, converted)
		}
	}

	log.Printf("Client: Converted %d conversations", len(convs))
	c.store.SetConversations(convs)
	return convs, nil
}

// GetMessages fetches messages for a conversation
func (c *Client) GetMessages(ctx context.Context, conversationID string) ([]*store.Message, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	resp, err := client.FetchMessages(conversationID, 50, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	msgs := convertMessages(resp.GetMessages(), conversationID)
	c.store.SetMessages(conversationID, msgs)

	return msgs, nil
}

// SendMessage sends a text message to a conversation
func (c *Client) SendMessage(ctx context.Context, conversationID string, text string) error {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("client not connected")
	}

	// Create the message request
	req := &gmproto.SendMessageRequest{
		ConversationID: conversationID,
		MessagePayload: &gmproto.MessagePayload{
			MessagePayloadContent: &gmproto.MessagePayloadContent{
				MessageContent: &gmproto.MessageContent{
					Content: text,
				},
			},
		},
	}

	_, err := client.SendMessage(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// MarkRead marks a conversation as read
func (c *Client) MarkRead(ctx context.Context, conversationID string, messageID string) error {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("client not connected")
	}

	if err := client.MarkRead(conversationID, messageID); err != nil {
		return fmt.Errorf("failed to mark as read: %w", err)
	}

	c.store.MarkConversationRead(conversationID)
	return nil
}

// SendReaction sends a reaction to a message
func (c *Client) SendReaction(ctx context.Context, conversationID string, messageID string, emoji string) error {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("client not connected")
	}

	req := &gmproto.SendReactionRequest{
		MessageID:    messageID,
		ReactionData: gmproto.MakeReactionData(emoji),
		Action:       gmproto.SendReactionRequest_ADD,
	}

	_, err := client.SendReaction(req)
	if err != nil {
		return fmt.Errorf("failed to send reaction: %w", err)
	}

	return nil
}

// Close closes the client and cleans up resources
func (c *Client) Close() {
	c.Disconnect()
	close(c.eventChan)
}
