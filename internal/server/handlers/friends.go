// internal/server/handlers/friends.go
package handlers

import (
	"fmt"
	"log"
	"textual/internal/server/database"
	"textual/pkg/protocol"
	"time"
)

type FriendHandler struct {
    db        *database.DB
    clients   map[string]*Client
    broadcast chan protocol.Message
}

func NewFriendHandler(db *database.DB, clients map[string]*Client, broadcast chan protocol.Message) *FriendHandler {
    return &FriendHandler{
        db:        db,
        clients:   clients,
        broadcast: broadcast,
    }
}

func (h *FriendHandler) HandleFriendRequest(senderID string, request protocol.FriendRequestPayload) error {
    // get sender info
    sender, err := h.db.GetUser(senderID)
    if err != nil {
        return fmt.Errorf("failed to get sender info: %v", err)
    }

    // get target user info
    targetUser, err := h.db.GetUserByUsername(request.ToUser)
    if err != nil {
        return fmt.Errorf("target user not found: %v", err)
    }

    // generate request ID
    requestID := fmt.Sprintf("fr-%s-%s-%d", sender.ID, targetUser.ID, time.Now().Unix())

    // Create friend request in database
    err = h.db.CreateFriendRequest(sender.ID, targetUser.ID)
    if err != nil {
        return fmt.Errorf("failed to create friend request: %v", err)
    }

    // prepare notification
    notification := protocol.Message{
        Type: protocol.TypeFriendRequest,
        Payload: protocol.FriendRequestPayload{
            RequestID: requestID,
            FromUser:  sender.Username,
            ToUser:    targetUser.Username,
            Status:    "pending",
        },
        Timestamp: time.Now().Unix(),
    }

    // send notification to target user if online
    if targetClient, ok := h.clients[targetUser.ID]; ok {
        log.Printf("Sending friend request notification to %s", targetUser.Username)
        select {
        case targetClient.Send <- notification:
            log.Printf("Friend request notification sent to %s", targetUser.Username)
        default:
            log.Printf("Failed to send friend request notification to %s: channel full", targetUser.Username)
        }
    }

    // send confirmation to sender
    if senderClient, ok := h.clients[senderID]; ok {
        confirmation := protocol.Message{
            Type: protocol.TypeFriendRequest,
            Payload: protocol.FriendRequestPayload{
                RequestID: requestID,
                FromUser:  sender.Username,
                ToUser:    targetUser.Username,
                Status:    "sent",
            },
            Timestamp: time.Now().Unix(),
        }
        select {
        case senderClient.Send <- confirmation:
            log.Printf("Friend request confirmation sent to %s", sender.Username)
        default:
            log.Printf("Failed to send friend request confirmation to %s: channel full", sender.Username)
        }
    }

    return nil
}

func (h *FriendHandler) HandleFriendResponse(userID string, response protocol.FriendResponsePayload) error {
    // get users involved in the friend request
    fromUser, toUser, err := h.db.GetFriendRequestUsers(response.RequestID)
    if err != nil {
        return fmt.Errorf("failed to get friend request: %v", err)
    }

    // verify that the user is the intended recipient of the response
    if userID != toUser.ID {
        return fmt.Errorf("unauthorized to respond to this friend request")
    }

    // update the db
    if response.Accept {
        err = h.db.AcceptFriendRequest(fromUser.ID, toUser.ID)
    } else {
        err = h.db.RejectFriendRequest(fromUser.ID, toUser.ID)
    }

    if err != nil {
        return fmt.Errorf("failed to update friend request: %v", err)
    }

    // prepare notification
    status := "rejected"
    if response.Accept {
        status = "accepted"
    }

    notification := protocol.Message{
        Type: protocol.TypeFriendResponse,
        Payload: protocol.FriendResponsePayload{
            RequestID: response.RequestID,
            FromUser:  toUser.Username,  // Responder
            Accept:    response.Accept,
        },
        Timestamp: time.Now().Unix(),
    }

    // notify the requester
    if requesterClient, ok := h.clients[fromUser.ID]; ok {
        select {
        case requesterClient.Send <- notification:
            log.Printf("Friend request response sent to %s: %s", fromUser.Username, status)
        default:
            log.Printf("Failed to send friend request response to %s: channel full", fromUser.Username)
        }
    }

    // send confirmation to responder
    if responderClient, ok := h.clients[toUser.ID]; ok {
        confirmation := protocol.Message{
            Type: protocol.TypeFriendResponse,
            Payload: protocol.FriendResponsePayload{
                RequestID: response.RequestID,
                FromUser:  fromUser.Username,  // Original requester
                Accept:    response.Accept,
            },
            Timestamp: time.Now().Unix(),
        }
        select {
        case responderClient.Send <- confirmation:
            log.Printf("Friend request response confirmation sent to %s", toUser.Username)
        default:
            log.Printf("Failed to send friend request response confirmation to %s: channel full", toUser.Username)
        }
    }

    // if accepted, update friend list fot both users
    if response.Accept {
        h.sendUpdatedFriendList(fromUser.ID)
        h.sendUpdatedFriendList(toUser.ID)
    }

    return nil
}

func (h *FriendHandler) sendUpdatedFriendList(userID string) error {
    // get friend list
    friends, err := h.db.GetFriends(userID)
    if err != nil {
        return fmt.Errorf("failed to get friend list: %v", err)
    }

    // prepare friend info list
    friendInfos := make([]protocol.UserInfo, 0, len(friends))
    for _, friend := range friends {
        friendInfos = append(friendInfos, protocol.UserInfo{
            ID:       friend.ID,
            Username: friend.Username,
            Status:   friend.Status,
        })
    }

    // send updated friend list to user
    if client, ok := h.clients[userID]; ok {
        msg := protocol.Message{
            Type: protocol.TypeFriendList,
            Payload: protocol.FriendListPayload{
                Friends: friendInfos,
            },
            Timestamp: time.Now().Unix(),
        }
        select {
        case client.Send <- msg:
            log.Printf("Updated friend list sent to user %s", userID)
        default:
            log.Printf("Failed to send updated friend list to user %s: channel full", userID)
        }
    }

    return nil
}



func (h *FriendHandler) SendFriendRequest(userID, friendUsername string) error {
    friend, err := h.db.GetUser(friendUsername)
    if err != nil {
        return err
    }
    
    return h.db.CreateFriendRequest(userID, friend.ID)
}

func (h *FriendHandler) AcceptFriendRequest(userID, friendID string) error {
    return h.db.AcceptFriendRequest(userID, friendID)
}

func (h *FriendHandler) GetFriendList(userID string) ([]string, error) {
    return h.db.GetFriendList(userID)
}

