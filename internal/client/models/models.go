// internal/client/models/models.go
package models

import (
	"net"
	"textual/pkg/protocol"
	"time"
)


type Message struct {
    ID          string     `json:"id"`
    Content     string     `json:"content"`
    SenderID    string     `json:"sender_id"`
    RecipientID *string    `json:"recipient_id,omitempty"`
    GroupID     *string    `json:"group_id,omitempty"`
    SentAt      time.Time  `json:"sent_at"`
    ReadAt      *time.Time `json:"read_at,omitempty"`
    Read        bool       `json:"read"`
    SenderName  string     `json:"sender_name,omitempty"`
}


type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Status   string `json:"status"`
}


type Group struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedBy   string    `json:"created_by"`
    CreatedAt   time.Time `json:"created_at"`
    Members     []string  `json:"members"`
}


type FriendRequest struct {
    ID        string    `json:"id"`
    FromUser  string    `json:"from_user"`
    ToUser    string    `json:"to_user"`
    Status    string    `json:"status"` // "pending", "accepted", "rejected"
    CreatedAt time.Time `json:"created_at"`
}


type (

    NewMessage struct {
        Content     string
        RecipientID *string
        GroupID     *string
    }


    MessageReceived struct {
        Message Message
    }


    StatusUpdate struct {
        UserID string
        Status string
    }


    ErrorOccurred struct {
        Error error
    }


    FriendRequestReceived struct {
        Request FriendRequest
    }


    GroupInviteReceived struct {
        Group Group
    }
)

// status
const (
    StatusOnline  = "online"
    StatusAway    = "away"
    StatusOffline = "offline"
)

// type of message
const (
    MessageTypeGlobal = "global"
    MessageTypeDirect = "direct"
    MessageTypeGroup  = "group"
)

// direct message
func NewDirectMessage(content string, recipientID string) NewMessage {
    return NewMessage{
        Content:     content,
        RecipientID: &recipientID,
    }
}

// group message
func NewGroupMessage(content string, groupID string) NewMessage {
    return NewMessage{
        Content:  content,
        GroupID:  &groupID,
    }
}


func (m *Message) IsDirect() bool {
    return m.RecipientID != nil
}


func (m *Message) IsGroup() bool {
    return m.GroupID != nil
}


func (m *Message) IsGlobal() bool {
    return m.RecipientID == nil && m.GroupID == nil
}


func (m *Message) GetChatID() string {
    if m.GroupID != nil {
        return *m.GroupID
    }
    if m.RecipientID != nil {
        return *m.RecipientID
    }
    return "global"
}


func (u *User) IsOnline() bool {
    return u.Status == StatusOnline
}


func (fr *FriendRequest) IsPending() bool {
    return fr.Status == "pending"
}


func (fr *FriendRequest) Accept() {
    fr.Status = "accepted"
}


func (fr *FriendRequest) Reject() {
    fr.Status = "rejected"
}


func (g *Group) HasMember(userID string) bool {
    for _, member := range g.Members {
        if member == userID {
            return true
        }
    }
    return false
}


func (g *Group) AddMember(userID string) {
    if !g.HasMember(userID) {
        g.Members = append(g.Members, userID)
    }
}


func (g *Group) RemoveMember(userID string) {
    for i, member := range g.Members {
        if member == userID {
            g.Members = append(g.Members[:i], g.Members[i+1:]...)
            break
        }
    }
}

type Client struct {
    ID       string
    Username string
    Conn     net.Conn
    Send     chan protocol.Message
}


type ErrorMsg struct {
	Error string `json:"error"`
}
