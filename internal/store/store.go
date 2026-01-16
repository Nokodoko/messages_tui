package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/n0ko/messages-tui/internal/config"
)

// Session holds the authentication session data
type Session struct {
	// DevicePair contains the pairing data from libgm
	DevicePair json.RawMessage `json:"device_pair"`
	// Browser contains browser registration data
	Browser json.RawMessage `json:"browser"`
	// CreatedAt is when the session was created
	CreatedAt time.Time `json:"created_at"`
	// LastUsed is when the session was last used
	LastUsed time.Time `json:"last_used"`
}

// Conversation represents a cached conversation
type Conversation struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	LatestMessage   string    `json:"latest_message"`
	LatestTimestamp time.Time `json:"latest_timestamp"`
	Unread          bool      `json:"unread"`
	IsGroup         bool      `json:"is_group"`
	Participants    []string  `json:"participants"`
	AvatarURL       string    `json:"avatar_url"`
}

// Message represents a cached message
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Content        string    `json:"content"`
	Timestamp      time.Time `json:"timestamp"`
	IsFromMe       bool      `json:"is_from_me"`
	Status         string    `json:"status"` // sent, delivered, read, failed
	Reactions      []string  `json:"reactions"`
	MediaURL       string    `json:"media_url"`
	MediaType      string    `json:"media_type"`
}

// Store manages session and message caching
type Store struct {
	mu            sync.RWMutex
	session       *Session
	conversations map[string]*Conversation
	messages      map[string][]*Message // keyed by conversation ID
}

// New creates a new Store instance
func New() *Store {
	return &Store{
		conversations: make(map[string]*Conversation),
		messages:      make(map[string][]*Message),
	}
}

// sessionPath returns the path to the session file
func sessionPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// LoadSession loads the session from disk
func (s *Store) LoadSession() (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := sessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	s.session = &session
	return &session, nil
}

// SaveSession saves the session to disk
func (s *Store) SaveSession(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := config.EnsureConfigDir(); err != nil {
		return err
	}

	path, err := sessionPath()
	if err != nil {
		return err
	}

	session.LastUsed = time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	s.session = session
	return os.WriteFile(path, data, 0600)
}

// ClearSession removes the session from disk
func (s *Store) ClearSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.session = nil

	path, err := sessionPath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// HasSession checks if a session exists
func (s *Store) HasSession() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session != nil
}

// GetSession returns the current session
func (s *Store) GetSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

// SetConversations updates the conversation list
func (s *Store) SetConversations(convs []*Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversations = make(map[string]*Conversation)
	for _, c := range convs {
		s.conversations[c.ID] = c
	}
}

// GetConversations returns all conversations sorted by latest timestamp
func (s *Store) GetConversations() []*Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	convs := make([]*Conversation, 0, len(s.conversations))
	for _, c := range s.conversations {
		convs = append(convs, c)
	}

	// Sort by latest timestamp (newest first)
	for i := 0; i < len(convs)-1; i++ {
		for j := i + 1; j < len(convs); j++ {
			if convs[j].LatestTimestamp.After(convs[i].LatestTimestamp) {
				convs[i], convs[j] = convs[j], convs[i]
			}
		}
	}

	return convs
}

// GetConversation returns a specific conversation
func (s *Store) GetConversation(id string) *Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conversations[id]
}

// UpdateConversation updates a single conversation
func (s *Store) UpdateConversation(conv *Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations[conv.ID] = conv
}

// SetMessages sets messages for a conversation
func (s *Store) SetMessages(conversationID string, msgs []*Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[conversationID] = msgs
}

// GetMessages returns messages for a conversation
func (s *Store) GetMessages(conversationID string) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages[conversationID]
}

// AddMessage adds a message to a conversation
func (s *Store) AddMessage(msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := s.messages[msg.ConversationID]
	s.messages[msg.ConversationID] = append(msgs, msg)

	// Update conversation's latest message
	if conv, ok := s.conversations[msg.ConversationID]; ok {
		conv.LatestMessage = msg.Content
		conv.LatestTimestamp = msg.Timestamp
		if !msg.IsFromMe {
			conv.Unread = true
		}
	}
}

// MarkConversationRead marks a conversation as read
func (s *Store) MarkConversationRead(conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conv, ok := s.conversations[conversationID]; ok {
		conv.Unread = false
	}
}
