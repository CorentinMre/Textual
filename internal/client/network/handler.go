// internal/client/network/handler.go
package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"textual/internal/client/models"
	"textual/pkg/protocol"
	"time"
)

type ConnectionHandler struct {
    conn         net.Conn
    sendChan     chan protocol.Message
    onMessage    func(models.Message)
    onLoadedMessages func([]models.Message)
    onError      func(error)
    onConnect    func()
    onDisconnect func()
    done         chan struct{}
    closeOnce    sync.Once
    mu           sync.RWMutex
    authComplete bool
    userID       string
    authError error
}

func NewConnectionHandler(conn net.Conn) *ConnectionHandler {
    return &ConnectionHandler{
        conn:         conn,
        sendChan:     make(chan protocol.Message, 100),
        done:         make(chan struct{}),
        authComplete: false,
    }
}

func (h *ConnectionHandler) Start() {
    log.Printf("Starting connection handler")
    go h.readLoop()
    go h.writeLoop()

    if h.onConnect != nil {
        h.onConnect()
    }
}





func (h *ConnectionHandler) readLoop() {
    defer h.handleDisconnect()

    decoder := json.NewDecoder(h.conn)
    for {
        select {
        case <-h.done:
            return
        default:
            var msg protocol.Message
            if err := decoder.Decode(&msg); err != nil {
                if err != io.EOF {
                    log.Printf("Read error: %v", err)
                    if h.onError != nil {
                        h.onError(fmt.Errorf("read error: %v", err))
                    }
                }
                return
            }

            log.Printf("Received message type: %s", msg.Type)
            h.handleMessage(msg)
        }
    }
}

func (h *ConnectionHandler) writeLoop() {
    encoder := json.NewEncoder(h.conn)
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-h.done:
            return
        case msg := <-h.sendChan:
            h.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := encoder.Encode(msg); err != nil {
                log.Printf("Write error: %v", err)
                if h.onError != nil {
                    h.onError(fmt.Errorf("write error: %v", err))
                }
                return
            }
            log.Printf("Successfully sent message type: %s", msg.Type)
        case <-ticker.C:
            h.mu.RLock()
            isAuth := h.authComplete
            h.mu.RUnlock()
            
            if isAuth {
                h.sendMessage(protocol.Message{
                    Type:      protocol.TypePing,
                    Timestamp: time.Now().Unix(),
                })
            }
        }
    }
}

func (h *ConnectionHandler) handleMessage(msg protocol.Message) {
    switch msg.Type {
    case protocol.TypeFriendRequest:
        var friendReq protocol.FriendRequestPayload
        data, err := json.Marshal(msg.Payload)
        if err != nil {
            log.Printf("Error marshaling friend request payload: %v", err)
            return
        }
        
        if err := json.Unmarshal(data, &friendReq); err != nil {
            log.Printf("Error unmarshaling friend request payload: %v", err)
            return
        }

        if h.onMessage != nil {
            // friend request message
            h.onMessage(models.Message{
                ID:         friendReq.RequestID,
                Content:    "Friend request",
                SenderID:   friendReq.FromUser,
                SenderName: friendReq.FromUser,
                SentAt:    time.Now(),
            })
        }
    case protocol.TypeMessageHistory:
        data, err := json.Marshal(msg.Payload)
        if err != nil {
            log.Printf("Failed to marshal message history payload: %v", err)
            return
        }
        
        var historyPayload struct {
            Messages []models.Message `json:"messages"`
        }
        
        if err := json.Unmarshal(data, &historyPayload); err != nil {
            log.Printf("Failed to unmarshal message history: %v", err)
            return
        }
        
        for _, msg := range historyPayload.Messages {
            if h.onMessage != nil {
                h.onMessage(msg)
            }
        }
        
    case protocol.TypeAuthResponse:
        h.handleAuthResponse(msg)
        
    case protocol.TypeDirectMessage, protocol.TypeGroupMessage, protocol.TypeGlobalMessage:
        h.mu.RLock()
        isAuth := h.authComplete
        h.mu.RUnlock()
        
        if !isAuth {
            log.Printf("Received message but not authenticated")
            return
        }
        
        if modelMsg, err := h.convertToModelMessage(msg); err == nil {
            log.Printf("Converted message: %+v", modelMsg)
            if h.onMessage != nil {
                h.onMessage(modelMsg)
            }
        } else {
            log.Printf("Failed to convert message: %v", err)
        }
        
    case protocol.TypePong:
        // Ignore pong messages
        
    default:
        log.Printf("Received unknown message type: %s", msg.Type)
    }
}



// func (h *ConnectionHandler) convertToModelMessage(msg protocol.Message) (models.Message, error) {
//     modelMsg := models.Message{
//         Timestamp: time.Unix(msg.Timestamp, 0),
//     }

//     payload, ok := msg.Payload.(map[string]interface{})
//     if !ok {
//         return modelMsg, fmt.Errorf("invalid payload format")
//     }

//     if content, ok := payload["content"].(string); ok {
//         modelMsg.Content = content
//     }
//     if senderID, ok := payload["sender_id"].(string); ok {
//         modelMsg.SenderID = senderID
//     }
//     if recipientID, ok := payload["recipient_id"].(string); ok {
//         modelMsg.RecipientID = &recipientID
//     }
//     if groupID, ok := payload["group_id"].(string); ok {
//         modelMsg.GroupID = &groupID
//     }
//     if SenderName, ok := payload["sender_name"].(string); ok {
//         modelMsg.SenderName = SenderName
//     }

//     return modelMsg, nil
// }


func (h *ConnectionHandler) SetMessageHandler(handler func(models.Message)) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.onMessage = handler
}

func (h *ConnectionHandler) SetErrorHandler(handler func(error)) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.onError = handler
}

func (h *ConnectionHandler) SetConnectHandler(handler func()) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.onConnect = handler
}

func (h *ConnectionHandler) SetDisconnectHandler(handler func()) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.onDisconnect = handler
}

func (h *ConnectionHandler) handleDisconnect() {
    h.closeOnce.Do(func() {
        close(h.done)
        h.conn.Close()
        if h.onDisconnect != nil {
            h.onDisconnect()
        }
    })
}

func (h *ConnectionHandler) Close() error {
    h.handleDisconnect()
    return nil
}


func (h *ConnectionHandler) handleAuthResponse(msg protocol.Message) {
    log.Printf("Processing auth response: %+v", msg)

    var authResp protocol.AuthResponsePayload
    data, err := json.Marshal(msg.Payload)
    if err != nil {
        log.Printf("Failed to marshal auth payload: %v", err)
        h.setAuthError(err)
        return
    }

    if err := json.Unmarshal(data, &authResp); err != nil {
        log.Printf("Failed to unmarshal auth payload: %v", err)
        h.setAuthError(err)
        return
    }

    h.mu.Lock()
    defer h.mu.Unlock()
    
    if authResp.Success {
        h.authComplete = true
        h.userID = authResp.UserID
        h.authError = nil
        log.Printf("Authentication successful. UserID: %s", h.userID)
    } else {
        h.authComplete = false
        h.authError = fmt.Errorf("authentication failed: %s", authResp.Error)
        if h.onError != nil {
            h.onError(h.authError)
        }
        log.Printf("Authentication failed: %s", authResp.Error)
    }
}

func (h *ConnectionHandler) setAuthError(err error) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.authError = err
}

func (h *ConnectionHandler) GetAuthError() error {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.authError
}

func (h *ConnectionHandler) SendAuthRequest(username, password string) error {
    log.Printf("Sending auth request for user: %s", username)
    
    authReq := protocol.Message{
        Type: protocol.TypeAuth,
        Payload: protocol.AuthPayload{
            Username: username,
            Password: password,
        },
        Timestamp: time.Now().Unix(),
    }

    h.mu.Lock()
    h.authComplete = false // Reset auth state
    h.mu.Unlock()

    return h.sendMessage(authReq)
}

func (h *ConnectionHandler) IsAuthenticated() bool {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.authComplete
}

func (h *ConnectionHandler) sendMessage(msg protocol.Message) error {
    log.Printf("Sending message type: %s", msg.Type)
    select {
    case h.sendChan <- msg:
        return nil
    case <-h.done:
        return fmt.Errorf("connection closed")
    case <-time.After(5 * time.Second):
        return fmt.Errorf("send timeout")
    }
}

func (h *ConnectionHandler) SendMessage(content string, recipientID *string, groupID *string) error {
    if !h.IsAuthenticated() {
        log.Printf("Attempting to send message without authentication")
        return fmt.Errorf("not authenticated")
    }

    log.Printf("Preparing to send message. Auth state: %v", h.IsAuthenticated())

    var msg protocol.Message
    if recipientID != nil {
        msg = protocol.NewDirectMessage(content, h.userID, "", *recipientID)
    } else if groupID != nil {
        msg = protocol.NewGroupMessage(content, h.userID, "", *groupID)
    } else {
        msg = protocol.NewGlobalMessage(content, h.userID, "")
    }

    return h.sendMessage(msg)
}


func (h *ConnectionHandler) convertToModelMessage(msg protocol.Message) (models.Message, error) {
    modelMsg := models.Message{
        SentAt: time.Unix(msg.Timestamp, 0),
    }

    payload, ok := msg.Payload.(map[string]interface{})
    if !ok {
        return modelMsg, fmt.Errorf("invalid payload format")
    }

    if id, ok := payload["id"].(string); ok {
        modelMsg.ID = id
    }
    if content, ok := payload["content"].(string); ok {
        modelMsg.Content = content
    }
    if senderID, ok := payload["sender_id"].(string); ok {
        modelMsg.SenderID = senderID
    }
    if recipientID, ok := payload["recipient_id"].(string); ok {
        modelMsg.RecipientID = &recipientID
    }
    if groupID, ok := payload["group_id"].(string); ok {
        modelMsg.GroupID = &groupID
    }
    if senderName, ok := payload["sender_name"].(string); ok {
        modelMsg.SenderName = senderName
    }
    if sentAt, ok := payload["sent_at"].(float64); ok {
        modelMsg.SentAt = time.Unix(int64(sentAt), 0)
    }
    if readAt, ok := payload["read_at"].(float64); ok {
        t := time.Unix(int64(readAt), 0)
        modelMsg.ReadAt = &t
    }

    return modelMsg, nil
}

func (h *ConnectionHandler) SetLoadedMessagesHandler(handler func([]models.Message)) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.onLoadedMessages = handler
}

func (h *ConnectionHandler) LoadMessages(beforeID string, limit int) error {
    if !h.IsAuthenticated() {
        return fmt.Errorf("not authenticated")
    }

    msg := protocol.NewLoadMessagesRequest(beforeID, limit)
    return h.sendMessage(msg)
}

func (h *ConnectionHandler) SendFriendRequest(username string) error {
    msg := protocol.NewMessage(protocol.TypeFriendRequest, protocol.FriendRequestPayload{
        ToUser: username,
    })
    return h.sendMessage(msg)
}

func (h *ConnectionHandler) AcceptFriendRequest(requestID string) error {
    msg := protocol.NewMessage(protocol.TypeFriendResponse, protocol.FriendResponsePayload{
        RequestID: requestID,
        Accept:    true,
    })
    return h.sendMessage(msg)
}

func (h *ConnectionHandler) RemoveFriend(friendID string) error {
    msg := protocol.NewMessage(protocol.TypeFriendRemove, map[string]string{
        "friend_id": friendID,
    })
    return h.sendMessage(msg)
}


func (h *ConnectionHandler) CreateGroup(name, description string) error {
    if !h.IsAuthenticated() {
        return fmt.Errorf("not authenticated")
    }

    msg := protocol.NewMessage(protocol.TypeGroupCreate, protocol.GroupCreatePayload{
        Name:        name,
        Description: description,
    })

    return h.sendMessage(msg)
}

func (h *ConnectionHandler) LoadGroups() error {
    if !h.IsAuthenticated() {
        return fmt.Errorf("not authenticated")
    }

    msg := protocol.NewMessage(protocol.TypeGroupList, nil)
    return h.sendMessage(msg)
}


func (h *ConnectionHandler) JoinGroup(groupID string) error {
    if !h.IsAuthenticated() {
        return fmt.Errorf("not authenticated")
    }

    msg := protocol.NewMessage(protocol.TypeGroupJoin, protocol.GroupJoinPayload{
        GroupID: groupID,
        UserID:  h.userID,
    })

    return h.sendMessage(msg)
}


func (h *ConnectionHandler) LeaveGroup(groupID string) error {
    if !h.IsAuthenticated() {
        return fmt.Errorf("not authenticated")
    }

    msg := protocol.NewMessage(protocol.TypeGroupLeave, protocol.GroupJoinPayload{
        GroupID: groupID,
        UserID:  h.userID,
    })

    return h.sendMessage(msg)
}
