// internal/server/handlers/events.go
package handlers

import (
	"sync"
	"textual/pkg/protocol"
	"time"
)

type EventHandler struct {
    subscribers map[string]chan protocol.Message
    mu          sync.RWMutex
}

func NewEventHandler() *EventHandler {
    return &EventHandler{
        subscribers: make(map[string]chan protocol.Message),
    }
}

func (h *EventHandler) Subscribe(userID string) chan protocol.Message {
    h.mu.Lock()
    defer h.mu.Unlock()

    ch := make(chan protocol.Message, 100)
    h.subscribers[userID] = ch
    return ch
}

func (h *EventHandler) Unsubscribe(userID string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if ch, ok := h.subscribers[userID]; ok {
        close(ch)
        delete(h.subscribers, userID)
    }
}

func (h *EventHandler) Broadcast(msg protocol.Message) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for _, ch := range h.subscribers {
        select {
        case ch <- msg:
        default:
            // Si le canal est plein, on ignore le message pour ce subscriber
        }
    }
}

func (h *EventHandler) BroadcastToGroup(groupID string, msg protocol.Message) {
    // À implémenter : envoyer uniquement aux membres du groupe
}

func (h *EventHandler) NotifyUser(userID string, msg protocol.Message) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    if ch, ok := h.subscribers[userID]; ok {
        select {
        case ch <- msg:
        default:
            // Si le canal est plein, on ignore le message
        }
    }
}

type StatusMonitor struct {
    eventHandler *EventHandler
    userStatus   map[string]string
    lastSeen    map[string]time.Time
    mu          sync.RWMutex
}

func NewStatusMonitor(eh *EventHandler) *StatusMonitor {
    sm := &StatusMonitor{
        eventHandler: eh,
        userStatus:   make(map[string]string),
        lastSeen:    make(map[string]time.Time),
    }

    // Démarrer le moniteur de statut
    go sm.monitor()
    return sm
}

func (sm *StatusMonitor) monitor() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        sm.checkInactiveUsers()
    }
}

func (sm *StatusMonitor) checkInactiveUsers() {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    now := time.Now()
    for userID, lastSeen := range sm.lastSeen {
        if now.Sub(lastSeen) > 5*time.Minute && sm.userStatus[userID] == "online" {
            sm.userStatus[userID] = "away"
            sm.eventHandler.Broadcast(protocol.NewMessage(protocol.TypeStatusUpdate, protocol.StatusUpdatePayload{
                UserID: userID,
                Status: "away",
            }))
        }
    }
}

func (sm *StatusMonitor) UpdateStatus(userID, status string) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if oldStatus, exists := sm.userStatus[userID]; !exists || oldStatus != status {
        sm.userStatus[userID] = status
        sm.lastSeen[userID] = time.Now()

        sm.eventHandler.Broadcast(protocol.NewMessage(protocol.TypeStatusUpdate, protocol.StatusUpdatePayload{
            UserID: userID,
            Status: status,
        }))
    }
}

func (sm *StatusMonitor) UpdateLastSeen(userID string) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    sm.lastSeen[userID] = time.Now()
}