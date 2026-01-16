package client

import (
	"time"

	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"

	"github.com/n0ko/messages-tui/internal/store"
)

// EventType represents the type of client event
type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeConnected
	EventTypeDisconnected
	EventTypeNewMessage
	EventTypeMessageUpdated
	EventTypeConversationsUpdated
	EventTypeTypingIndicator
	EventTypeReadReceipt
	EventTypeError
)

// Event represents a client event that can be sent to the UI
type Event struct {
	Type         EventType
	Conversation *store.Conversation
	Message      *store.Message
	Error        error
	Data         any
}

// convertConversation converts a libgm conversation to our store format
func convertConversation(conv *gmproto.Conversation) *store.Conversation {
	if conv == nil {
		return nil
	}

	name := conv.GetName()
	if name == "" && len(conv.GetParticipants()) > 0 {
		// Use first participant's name if no conversation name
		for _, p := range conv.GetParticipants() {
			if p.GetFormattedNumber() != "" {
				name = p.GetFormattedNumber()
				break
			}
		}
	}

	var latestMsg string
	var latestTime time.Time

	// Get latest message content from LatestMessage.DisplayContent
	if latest := conv.GetLatestMessage(); latest != nil {
		latestMsg = latest.GetDisplayContent()
	}

	// Get timestamp from conversation's LastMessageTimestamp
	if ts := conv.GetLastMessageTimestamp(); ts > 0 {
		latestTime = time.UnixMicro(ts)
	}

	participants := make([]string, 0, len(conv.GetParticipants()))
	for _, p := range conv.GetParticipants() {
		if p.GetFormattedNumber() != "" {
			participants = append(participants, p.GetFormattedNumber())
		}
	}

	return &store.Conversation{
		ID:              conv.GetConversationID(),
		Name:            name,
		LatestMessage:   latestMsg,
		LatestTimestamp: latestTime,
		Unread:          conv.GetUnread(),
		IsGroup:         conv.GetIsGroupChat(),
		Participants:    participants,
	}
}

// convertMessage converts a libgm message to our store format
func convertMessage(msg *gmproto.Message, conversationID string) *store.Message {
	if msg == nil {
		return nil
	}

	var timestamp time.Time
	if ts := msg.GetTimestamp(); ts > 0 {
		timestamp = time.UnixMicro(ts)
	}

	// Get content from MessageInfo using GetMessageContent()
	content := ""
	if infos := msg.GetMessageInfo(); len(infos) > 0 {
		for _, info := range infos {
			if msgContent := info.GetMessageContent(); msgContent != nil {
				content = msgContent.GetContent()
				break
			}
		}
	}

	// Determine message status
	status := "sent"
	if msgStatus := msg.GetMessageStatus(); msgStatus != nil {
		switch msgStatus.GetStatus() {
		case gmproto.MessageStatusType_OUTGOING_DELIVERED:
			status = "delivered"
		case gmproto.MessageStatusType_OUTGOING_DISPLAYED:
			status = "read"
		case gmproto.MessageStatusType_OUTGOING_FAILED_GENERIC,
			gmproto.MessageStatusType_OUTGOING_FAILED_EMERGENCY_NUMBER:
			status = "failed"
		}
	}

	// Get sender info
	senderID := msg.GetParticipantID()
	senderName := ""
	if sender := msg.GetSenderParticipant(); sender != nil {
		senderName = sender.GetFormattedNumber()
	}

	// Determine if from me (outgoing messages have status >= 100)
	isFromMe := false
	if msgStatus := msg.GetMessageStatus(); msgStatus != nil {
		isFromMe = msgStatus.GetStatus().Number() >= 100
	}

	return &store.Message{
		ID:             msg.GetMessageID(),
		ConversationID: conversationID,
		SenderID:       senderID,
		SenderName:     senderName,
		Content:        content,
		Timestamp:      timestamp,
		IsFromMe:       isFromMe,
		Status:         status,
	}
}

// convertMessages converts a slice of libgm messages
func convertMessages(msgs []*gmproto.Message, conversationID string) []*store.Message {
	result := make([]*store.Message, 0, len(msgs))
	for _, msg := range msgs {
		if converted := convertMessage(msg, conversationID); converted != nil {
			result = append(result, converted)
		}
	}
	return result
}
