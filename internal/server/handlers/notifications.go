// internal/server/handlers/notifications.go
package handlers

import (
	"sync"
	"textual/internal/server/database"
	"textual/pkg/protocol"
	"time"
)

type NotificationType string

const (
    NotifyNewMessage     NotificationType = "new_message"
    NotifyFriendRequest  NotificationType = "friend_request"
    NotifyGroupInvite    NotificationType = "group_invite"
)

type Notification struct {
    Type      NotificationType `json:"type"`
    UserID    string          `json:"user_id"`
    Data      interface{}     `json:"data"`
    Timestamp time.Time       `json:"timestamp"`
    Read      bool           `json:"read"`
}

type NotificationHandler struct {
    db           *database.DB
    eventHandler *EventHandler
    notifications map[string][]Notification
    mu           sync.RWMutex
}

func NewNotificationHandler(db *database.DB, eh *EventHandler) *NotificationHandler {
    return &NotificationHandler{
        db:            db,
        eventHandler:  eh,
        notifications: make(map[string][]Notification),
    }
}

func (h *NotificationHandler) AddNotification(userID string, notifType NotificationType, data interface{}) {
    h.mu.Lock()
    defer h.mu.Unlock()

    notification := Notification{
        Type:      notifType,
        UserID:    userID,
        Data:      data,
        Timestamp: time.Now(),
        Read:      false,
    }

    if _, ok := h.notifications[userID]; !ok {
        h.notifications[userID] = make([]Notification, 0)
    }

    h.notifications[userID] = append(h.notifications[userID], notification)

    // Notify the user
    h.eventHandler.NotifyUser(userID, protocol.NewMessage(protocol.TypeStatusUpdate, map[string]interface{}{
        "type":         "notification",
        "notification": notification,
    }))
}

func (h *NotificationHandler) GetUnreadCount(userID string) int {
    h.mu.RLock()
    defer h.mu.RUnlock()

    count := 0
    if notifications, ok := h.notifications[userID]; ok {
        for _, n := range notifications {
            if !n.Read {
                count++
            }
        }
    }
    return count
}

func (h *NotificationHandler) MarkAsRead(userID string, notifType NotificationType) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if notifications, ok := h.notifications[userID]; ok {
        for i := range notifications {
            if notifications[i].Type == notifType {
                notifications[i].Read = true
            }
        }
    }
}

func (h *NotificationHandler) GetNotifications(userID string) []Notification {
    h.mu.RLock()
    defer h.mu.RUnlock()

    if notifications, ok := h.notifications[userID]; ok {
        return notifications
    }
    return []Notification{}
}

func (h *NotificationHandler) CleanOldNotifications() {
    h.mu.Lock()
    defer h.mu.Unlock()

    cutoff := time.Now().Add(-7 * 24 * time.Hour) // keep notifications for 1 week
    for userID, notifications := range h.notifications {
        var recent []Notification
        for _, n := range notifications {
            if n.Timestamp.After(cutoff) {
                recent = append(recent, n)
            }
        }
        h.notifications[userID] = recent
    }
}
