// internal/server/models/models.go
package models

import (
    "time"
    "net"
)

type User struct {
    ID           string     `json:"id"`
    Username     string     `json:"username"`
    PasswordHash string     `json:"-"`
    Status       string     `json:"status"`
    LastSeen     time.Time  `json:"last_seen"`
    LastLogin    time.Time  `json:"last_login"`
    CreatedAt    time.Time  `json:"created_at"`
}

type Message struct {
    ID          string     `json:"id"`
    Content     string     `json:"content"`
    SenderID    string     `json:"sender_id"`
    RecipientID *string    `json:"recipient_id,omitempty"`
    GroupID     *string    `json:"group_id,omitempty"`
    Status      string     `json:"status"`
    SentAt      time.Time  `json:"sent_at"`
    ReadAt      *time.Time `json:"read_at,omitempty"`
    SenderName  string     `json:"sender_name,omitempty"`
    // Timestamp   time.Time  `json:"timestamp"`
}

type Group struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedBy   string    `json:"created_by"`
    CreatedAt   time.Time `json:"created_at"`
    Status      string    `json:"status"`
    Members     []string  `json:"members"`
}

type GroupMember struct {
    GroupID   string    `json:"group_id"`
    UserID    string    `json:"user_id"`
    Role      string    `json:"role"`
    JoinedAt  time.Time `json:"joined_at"`
}

type FriendRequest struct {
    ID           string    `json:"id"`
    FromUserID   string    `json:"from_user_id"`
    ToUserID     string    `json:"to_user_id"`
    FromUsername string    `json:"from_username"`
    ToUsername   string    `json:"to_username"`
    Status       string    `json:"status"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type Notification struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Type      string    `json:"type"`
    Content   string    `json:"content"`
    RelatedID string    `json:"related_id"`
    CreatedAt time.Time `json:"created_at"`
    ReadAt    *time.Time `json:"read_at,omitempty"`
}

// Client représente une connexion client active
type Client struct {
    ID       string    `json:"id"`
    UserID   string    `json:"user_id"`
    Username string    `json:"username"`
    Conn     net.Conn  `json:"-"`
    Send     chan []byte `json:"-"`
}

// Constantes pour les statuts
const (
    StatusOnline  = "online"
    StatusOffline = "offline"
    StatusAway    = "away"
)

// Constantes pour les statuts de messages
const (
    MessageStatusSent      = "sent"
    MessageStatusDelivered = "delivered"
    MessageStatusRead      = "read"
    MessageStatusDeleted   = "deleted"
)

// Constantes pour les statuts d'amis
const (
    FriendStatusPending  = "pending"
    FriendStatusAccepted = "accepted"
    FriendStatusRejected = "rejected"
    FriendStatusBlocked  = "blocked"
)

// Constantes pour les statuts de groupe
const (
    GroupStatusActive   = "active"
    GroupStatusArchived = "archived"
    GroupStatusDeleted  = "deleted"
)

// Constantes pour les rôles de groupe
const (
    GroupRoleAdmin  = "admin"
    GroupRoleMember = "member"
)

// Constantes pour les types de notifications
const (
    NotificationTypeFriendRequest = "friend_request"
    NotificationTypeGroupInvite   = "group_invite"
    NotificationTypeNewMessage    = "new_message"
)

// Méthodes utilitaires pour User
func (u *User) IsOnline() bool {
    return u.Status == StatusOnline
}

func (u *User) IsAway() bool {
    return u.Status == StatusAway
}

// Méthodes utilitaires pour Message
func (m *Message) IsRead() bool {
    return m.Status == MessageStatusRead
}

func (m *Message) IsDirectMessage() bool {
    return m.RecipientID != nil
}

func (m *Message) IsGroupMessage() bool {
    return m.GroupID != nil
}

// Méthodes utilitaires pour Group
func (g *Group) IsActive() bool {
    return g.Status == GroupStatusActive
}

func (g *Group) HasMember(userID string) bool {
    for _, member := range g.Members {
        if member == userID {
            return true
        }
    }
    return false
}

// Méthodes utilitaires pour FriendRequest
func (fr *FriendRequest) IsPending() bool {
    return fr.Status == FriendStatusPending
}