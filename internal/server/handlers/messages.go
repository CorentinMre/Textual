// internal/server/handlers/messages.go
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"textual/internal/server/database"
	"textual/internal/server/models"
	"textual/pkg/protocol"
	"time"
)

type MessageHandler struct {
    db        *database.DB
    broadcast chan<- protocol.Message
    clients   map[string]*Client
    mu        sync.RWMutex
}

func NewMessageHandler(db *database.DB, broadcast chan<- protocol.Message, clients map[string]*Client) *MessageHandler {
    return &MessageHandler{
        db:        db,
        broadcast: broadcast,
        clients:   clients,
    }
}

func (h *MessageHandler) HandleMessage(senderID string, msg protocol.Message) error {
    log.Printf("Handling message of type %s from user %s", msg.Type, senderID)

    h.mu.RLock()
    sender, exists := h.clients[senderID]
    h.mu.RUnlock()

    if !exists {
        return fmt.Errorf("sender not found")
    }

    switch msg.Type {
    case protocol.TypeLoadMessages:
        return h.handleLoadMessages(sender, msg)
    case protocol.TypeGlobalMessage:
        return h.handleGlobalMessage(sender, msg)
    case protocol.TypeDirectMessage:
        return h.handleDirectMessage(sender, msg)
    case protocol.TypeGroupMessage:
        return h.handleGroupMessage(sender, msg)
    case protocol.TypePing:
        return h.handlePing(sender)
    case protocol.TypeFriendRequest:
        var payload protocol.FriendRequestPayload
        if err := h.decodePayload(msg.Payload, &payload); err != nil {
            return fmt.Errorf("invalid friend request payload: %v", err)
        }
        return h.handleFriendRequest(sender, payload)
    default:
        log.Printf("Unknown message type received: %s", msg.Type)
        return fmt.Errorf("unknown message type: %s", msg.Type)
    }
}

func (h *MessageHandler) handleFriendRequest(sender *Client, payload protocol.FriendRequestPayload) error {
    log.Printf("Processing friend request from %s to %s", sender.Username, payload.ToUser)

    // get the target user
    targetUser, err := h.db.GetUserByUsername(payload.ToUser)
    if err != nil {
        errMsg := protocol.NewMessage(protocol.TypeError, map[string]interface{}{
            "error": fmt.Sprintf("User not found: %s", payload.ToUser),
        })
        sender.Send <- errMsg
        return fmt.Errorf("target user not found: %v", err)
    }

    requestID := fmt.Sprintf("fr-%s-%s-%d", sender.ID, targetUser.ID, time.Now().Unix())

    if err := h.db.CreateFriendRequest(sender.ID, targetUser.ID); err != nil {
        return fmt.Errorf("failed to create friend request: %v", err)
    }

    // Send to target user if online
    h.mu.RLock()
    if recipient, ok := h.clients[targetUser.ID]; ok {
        // Le message doit être du même format que dans models.MessageReceived
        notification := protocol.Message{
            Type: protocol.TypeGlobalMessage, // Pour que ce soit traité comme un message normal
            Payload: map[string]interface{}{
                "id":          requestID,
                "content":     "Friend request",
                "sender_id":   sender.ID,
                "sender_name": sender.Username,
                "sent_at":     time.Now().Unix(),
            },
            Timestamp: time.Now().Unix(),
        }

        select {
        case recipient.Send <- notification:
            log.Printf("Friend request sent to %s", targetUser.Username)
        default:
            log.Printf("Failed to send notification: channel full")
        }
    }
    h.mu.RUnlock()

    // Confirm to sender
    confirmationMsg := protocol.NewMessage(protocol.TypeFriendRequest, protocol.FriendRequestPayload{
        RequestID: requestID,
        FromUser:  sender.Username,
        ToUser:    targetUser.Username,
        Status:    "sent",
    })

    select {
    case sender.Send <- confirmationMsg:
        log.Printf("Request confirmation sent to %s", sender.Username)
    default:
        log.Printf("Failed to send confirmation: channel full")
    }

    return nil
}

func (h *MessageHandler) handleLoadMessages(sender *Client, msg protocol.Message) error {
    var payload protocol.LoadMessagesPayload
    if err := h.decodePayload(msg.Payload, &payload); err != nil {
        return fmt.Errorf("invalid load messages payload: %v", err)
    }

    messages, err := h.db.GetMessagesBeforeID(sender.ID, payload.BeforeID, payload.Limit)
    if err != nil {
        return fmt.Errorf("failed to load messages: %v", err)
    }

    response := protocol.NewMessage(protocol.TypeMessageHistory, map[string]interface{}{
        "messages": messages,
    })

    select {
    case sender.Send <- response:
        return nil
    default:
        return fmt.Errorf("failed to send loaded messages: channel full")
    }
}

func (h *MessageHandler) handleGlobalMessage(sender *Client, msg protocol.Message) error {
    var payload struct {
        Content string `json:"content"`
    }

    if err := h.decodePayload(msg.Payload, &payload); err != nil {
        return fmt.Errorf("failed to decode global message payload: %v", err)
    }

    if payload.Content == "" {
        return fmt.Errorf("empty message content")
    }

    // Save to database
    dbMsg := &models.Message{
        Content:    payload.Content,
        SenderID:   sender.ID,
        SenderName: sender.Username,
        SentAt:     time.Now(),
        Status:     models.MessageStatusSent,
    }

    if err := h.db.SaveMessage(dbMsg); err != nil {
        return fmt.Errorf("failed to save message: %v", err)
    }

    // broadcast message
    broadcastMsg := protocol.Message{
        Type:      protocol.TypeGlobalMessage,
        Payload:   h.createMessagePayload(dbMsg),
        Timestamp: time.Now().Unix(),
    }

    h.broadcast <- broadcastMsg
    return nil
}

func (h *MessageHandler) handleDirectMessage(sender *Client, msg protocol.Message) error {
    var payload struct {
        Content     string `json:"content"`
        RecipientID string `json:"recipient_id"`
    }

    if err := h.decodePayload(msg.Payload, &payload); err != nil {
        return fmt.Errorf("failed to decode direct message payload: %v", err)
    }

    if payload.Content == "" || payload.RecipientID == "" {
        return fmt.Errorf("invalid message content or recipient")
    }

    // Save to database
    dbMsg := &models.Message{
        Content:     payload.Content,
        SenderID:    sender.ID,
        SenderName:  sender.Username,
        RecipientID: &payload.RecipientID,
        SentAt:      time.Now(),
        Status:      models.MessageStatusSent,
    }

    if err := h.db.SaveMessage(dbMsg); err != nil {
        return fmt.Errorf("failed to save message: %v", err)
    }

    // Send to recipient
    h.mu.RLock()
    recipient, exists := h.clients[payload.RecipientID]
    h.mu.RUnlock()

    if !exists {
        return fmt.Errorf("recipient not found")
    }

    directMsg := protocol.Message{
        Type: protocol.TypeDirectMessage,
        Payload: h.createMessagePayload(dbMsg),
        Timestamp: time.Now().Unix(),
    }

    // Send to both recipient and sender
    select {
    case recipient.Send <- directMsg:
        log.Printf("Message sent to recipient %s", recipient.Username)
    default:
        log.Printf("Failed to send to recipient %s: channel full", recipient.Username)
    }

    select {
    case sender.Send <- directMsg:
        log.Printf("Message confirmation sent to sender %s", sender.Username)
    default:
        log.Printf("Failed to send confirmation to sender %s: channel full", sender.Username)
    }

    return nil
}

func (h *MessageHandler) handleGroupMessage(sender *Client, msg protocol.Message) error {
    var payload struct {
        Content string `json:"content"`
        GroupID string `json:"group_id"`
    }

    if err := h.decodePayload(msg.Payload, &payload); err != nil {
        return fmt.Errorf("failed to decode group message payload: %v", err)
    }

    // Check if user is member of the group
    isMember, err := h.db.IsGroupMember(sender.ID, payload.GroupID)
    if err != nil {
        return fmt.Errorf("failed to check group membership: %v", err)
    }
    if !isMember {
        return fmt.Errorf("user is not a member of this group")
    }

    // Save message
    dbMsg := &models.Message{
        Content:    payload.Content,
        SenderID:   sender.ID,
        SenderName: sender.Username,
        GroupID:    &payload.GroupID,
        SentAt:     time.Now(),
        Status:     models.MessageStatusSent,
    }

    if err := h.db.SaveMessage(dbMsg); err != nil {
        return fmt.Errorf("failed to save message: %v", err)
    }

    // Get group members and send message
    members, err := h.db.GetGroupMembers(payload.GroupID)
    if err != nil {
        return fmt.Errorf("failed to get group members: %v", err)
    }

    groupMsg := protocol.Message{
        Type:      protocol.TypeGroupMessage,
        Payload:   h.createMessagePayload(dbMsg),
        Timestamp: time.Now().Unix(),
    }

    h.mu.RLock()
    for _, memberID := range members {
        if client, ok := h.clients[memberID]; ok {
            select {
            case client.Send <- groupMsg:
                log.Printf("Group message sent to member %s", client.Username)
            default:
                log.Printf("Failed to send to member %s: channel full", client.Username)
            }
        }
    }
    h.mu.RUnlock()

    return nil
}

func (h *MessageHandler) handlePing(client *Client) error {
    pongMsg := protocol.NewMessage(protocol.TypePong, nil)
    select {
    case client.Send <- pongMsg:
        return nil
    default:
        return fmt.Errorf("failed to send pong: channel full")
    }
}

func (h *MessageHandler) createMessagePayload(msg *models.Message) map[string]interface{} {
    payload := map[string]interface{}{
        "id":          msg.ID,
        "content":     msg.Content,
        "sender_id":   msg.SenderID,
        "sender_name": msg.SenderName,
        "sent_at":     msg.SentAt.Unix(),
    }

    if msg.RecipientID != nil {
        payload["recipient_id"] = *msg.RecipientID
    }
    if msg.GroupID != nil {
        payload["group_id"] = *msg.GroupID
    }
    if msg.ReadAt != nil {
        payload["read_at"] = msg.ReadAt.Unix()
    }

    return payload
}

func (h *MessageHandler) decodePayload(payload interface{}, target interface{}) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %v", err)
    }
    
    if err := json.Unmarshal(data, target); err != nil {
        return fmt.Errorf("failed to unmarshal payload: %v", err)
    }
    
    return nil
}
