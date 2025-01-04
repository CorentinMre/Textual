// internal/server/handlers/auth.go
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"textual/internal/server/database"
	"textual/internal/server/models"
	"textual/pkg/protocol"
	"time"
)

type AuthHandler struct {
    db         *database.DB
    clients    map[string]*Client
    broadcast  chan<- protocol.Message
}

func NewAuthHandler(db *database.DB, clients map[string]*Client, broadcast chan<- protocol.Message) *AuthHandler {
    return &AuthHandler{
        db:        db,
        clients:   clients,
        broadcast: broadcast,
    }
}

func (h *AuthHandler) HandleAuth(conn net.Conn) (*models.User, error) {
    // Set a read deadline to prevent hanging
    conn.SetReadDeadline(time.Now().Add(30 * time.Second))
    
    decoder := json.NewDecoder(conn)
    var msg protocol.Message
    if err := decoder.Decode(&msg); err != nil {
        return nil, fmt.Errorf("failed to decode auth message: %v", err)
    }

    // Reset the deadline after successful read
    conn.SetReadDeadline(time.Time{})

    if msg.Type != protocol.TypeAuth {
        return nil, fmt.Errorf("expected auth message, got %s", msg.Type)
    }

    var authPayload protocol.AuthPayload
    if err := h.decodePayload(msg.Payload, &authPayload); err != nil {
        return nil, fmt.Errorf("invalid auth payload: %v", err)
    }

    // Authenticate user
    user, err := h.db.AuthenticateUser(authPayload.Username, authPayload.Password)
    if err != nil {
        errorResponse := protocol.NewMessage(protocol.TypeError, protocol.ErrorPayload{
            Code:    protocol.ErrCodeInvalidAuth,
            Message: "Authentication failed",
        })
        
        if err := h.sendResponse(conn, errorResponse); err != nil {
            log.Printf("Failed to send error response: %v", err)
        }
        return nil, fmt.Errorf("authentication failed: %v", err)
    }

    // Convert to models.User if not already
    modelUser := &models.User{
        ID:       user.ID,
        Username: user.Username,
        Status:   protocol.StatusOnline,
    }

    // Send success response
    response := protocol.NewMessage(protocol.TypeAuthResponse, protocol.AuthResponsePayload{
        Success: true,
        UserID:  modelUser.ID,
    })

    if err := h.sendResponse(conn, response); err != nil {
        return nil, fmt.Errorf("failed to send auth response: %v", err)
    }

    // Update user status
    if err := h.db.UpdateUserStatus(modelUser.ID, protocol.StatusOnline); err != nil {
        log.Printf("Failed to update user status: %v", err)
    }

    // Send initial data
    if err := h.sendInitialData(conn, modelUser.ID); err != nil {
        log.Printf("Failed to send initial data: %v", err)
    }

    // Notify other users
    statusUpdate := protocol.NewMessage(protocol.TypeStatusUpdate, protocol.StatusUpdatePayload{
        UserID: modelUser.ID,
        Status: protocol.StatusOnline,
    })
    h.broadcast <- statusUpdate

    return modelUser, nil
}

// func (h *AuthHandler) sendInitialData(conn net.Conn, userID string) error {
//     // Send friend list
//     friends, err := h.db.GetFriends(userID)
//     if err == nil {
//         // Create user info slice
//         friendInfos := make([]protocol.UserInfo, 0, len(friends))
//         for _, friend := range friends {
//             friendInfos = append(friendInfos, protocol.UserInfo{
//                 ID:       friend.ID,
//                 Username: friend.Username,
//                 Status:   friend.Status,
//             })
//         }

//         // Create and send friend list message
//         friendList := protocol.NewMessage(protocol.TypeFriendList, map[string]interface{}{
//             "friends": friendInfos,
//         })

//         if err := h.sendResponse(conn, friendList); err != nil {
//             return fmt.Errorf("failed to send friend list: %v", err)
//         }
//     }

//     // Send pending friend requests
//     requests, err := h.db.GetPendingFriendRequests(userID)
//     if err == nil {
//         for _, req := range requests {
//             requestPayload := map[string]interface{}{
//                 "request_id": req.ID,
//                 "from_user": req.FromUserID,
//                 "to_user":   req.ToUserID,
//                 "status":    req.Status,
//             }
//             requestMsg := protocol.NewMessage(protocol.TypeFriendRequest, requestPayload)
//             if err := h.sendResponse(conn, requestMsg); err != nil {
//                 log.Printf("Failed to send friend request: %v", err)
//             }
//         }
//     }

//     return nil
// }

func (h *AuthHandler) decodePayload(payload interface{}, target interface{}) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, target)
}

func (h *AuthHandler) sendResponse(conn net.Conn, msg protocol.Message) error {
    // Set write deadline
    conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    defer conn.SetWriteDeadline(time.Time{})

    encoder := json.NewEncoder(conn)
    return encoder.Encode(msg)
}

func (h *AuthHandler) HandleLogout(userID string) error {
    // Update user status in database
    if err := h.db.UpdateUserStatus(userID, protocol.StatusOffline); err != nil {
        return err
    }

    // Notify other users
    statusUpdate := protocol.NewMessage(protocol.TypeStatusUpdate, protocol.StatusUpdatePayload{
        UserID: userID,
        Status: protocol.StatusOffline,
    })
    h.broadcast <- statusUpdate

    // Remove client from active clients
    delete(h.clients, userID)

    return nil
}

func (h *AuthHandler) ValidateSession(userID string) error {
    user, err := h.db.GetUser(userID)
    if err != nil {
        return fmt.Errorf("invalid session: user not found")
    }

    if user.Status != protocol.StatusOnline {
        return fmt.Errorf("invalid session: user not online")
    }

    return nil
}

func (h *AuthHandler) sendInitialData(conn net.Conn, userID string) error {
    // Get message history
    messages, err := h.db.GetMessages(userID, 100)
    if err == nil {
        historyMsg := protocol.NewMessage(protocol.TypeMessageHistory, map[string]interface{}{
            "messages": messages,
        })
        if err := h.sendResponse(conn, historyMsg); err != nil {
            return fmt.Errorf("failed to send message history: %v", err)
        }
    }

    // Send friend list
    friends, err := h.db.GetFriends(userID)
    if err == nil {
        friendInfos := make([]protocol.UserInfo, 0, len(friends))
        for _, friend := range friends {
            friendInfos = append(friendInfos, protocol.UserInfo{
                ID:       friend.ID,
                Username: friend.Username,
                Status:   friend.Status,
            })
        }

        friendList := protocol.NewMessage(protocol.TypeFriendList, map[string]interface{}{
            "friends": friendInfos,
        })

        if err := h.sendResponse(conn, friendList); err != nil {
            return fmt.Errorf("failed to send friend list: %v", err)
        }
    }

    // Send pending friend requests
    requests, err := h.db.GetPendingFriendRequests(userID)
    if err == nil {
        log.Printf("Found %d pending friend requests for user %s", len(requests), userID)
        for _, req := range requests {
            notification := protocol.Message{
                Type: protocol.TypeFriendRequest,
                Payload: protocol.FriendRequestPayload{
                    RequestID: fmt.Sprintf("fr-%s-%s-%d", req.FromUserID, req.ToUserID, req.CreatedAt.Unix()),
                    FromUser:  req.FromUsername,
                    ToUser:    req.ToUsername,
                    Status:    "pending",
                },
                Timestamp: req.CreatedAt.Unix(),
            }

            if err := h.sendResponse(conn, notification); err != nil {
                log.Printf("Failed to send friend request notification: %v", err)
            } else {
                log.Printf("Sent friend request notification from %s to %s", req.FromUsername, req.ToUsername)
            }
        }
    } else {
        log.Printf("Error getting pending friend requests: %v", err)
    }

    return nil
}
