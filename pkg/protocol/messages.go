// pkg/protocol/messages.go
package protocol

import (
    "fmt"
    "time"
)

type MessageType string

const (
    TypeAuth         MessageType = "auth"
    TypeMessageHistory MessageType = "message_history"
    TypeLoadMessages   MessageType = "load_messages"
    TypeLoadMessagesResponse MessageType = "load_messages_response"
    TypeAuthResponse MessageType = "auth_response"
    TypeDirectMessage   MessageType = "direct_message"
    TypeGroupMessage    MessageType = "group_message"
    TypeGlobalMessage   MessageType = "global_message"
    TypeStatusUpdate    MessageType = "status_update"
    TypeNotification    MessageType = "notification"
    TypeFriendRequest   MessageType = "friend_request"
    TypeFriendResponse  MessageType = "friend_response"
    TypeFriendList      MessageType = "friend_list"
    TypeGroupCreate     MessageType = "group_create"
    TypeGroupJoin       MessageType = "group_join"
    TypeGroupLeave      MessageType = "group_leave"
    TypeGroupList       MessageType = "group_list"
    TypeGroupInvite     MessageType = "group_invite"
    TypePing           MessageType = "ping"
    TypePong           MessageType = "pong"
    TypeError          MessageType = "error"
    TypeFriendRemove    MessageType = "friend_remove"
)

// error codes
const (
    ErrCodeInvalidAuth     = 1000
    ErrCodeNotAuth         = 1001
    ErrCodeInvalidMessage  = 1002
    ErrCodeUserNotFound    = 1003
    ErrCodeGroupNotFound   = 1004
    ErrCodeAccessDenied    = 1005
    ErrCodeNotAuthorized   = 1006
    ErrCodeAlreadyExists   = 1007
    ErrCodeInvalidRequest  = 1008
    ErrCodeInternalError   = 1009
)



// error payload
type ErrorPayload struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}



type DirectMessagePayload struct {
    Content     string `json:"content"`
    SenderID    string `json:"sender_id"`
    RecipientID string `json:"recipient_id"`
}

type GroupMessagePayload struct {
    Content  string `json:"content"`
    SenderID string `json:"sender_id"`
    GroupID  string `json:"group_id"`
}

type GlobalMessagePayload struct {
    Content  string `json:"content"`
    SenderID string `json:"sender_id"`
}

type StatusUpdatePayload struct {
    UserID string `json:"user_id"`
    Status string `json:"status"`
}

type NotificationPayload struct {
    Type    string      `json:"type"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

type FriendRequestPayload struct {
    RequestID string `json:"request_id"`
    FromUser  string `json:"from_user"`
    ToUser    string `json:"to_user"`
    Status    string `json:"status"`
}

type FriendListPayload struct {
    Friends []UserInfo `json:"friends"`
}

type GroupCreatePayload struct {
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    MemberIDs   []string `json:"member_ids,omitempty"`
}

type GroupJoinPayload struct {
    GroupID string `json:"group_id"`
    UserID  string `json:"user_id"`
}

type GroupPayload struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedBy   string    `json:"created_by"`
    CreatedAt   int64     `json:"created_at"`
    MemberIDs   []string  `json:"member_ids"`
}

type GroupListPayload struct {
    Groups []GroupPayload `json:"groups"`
}

type GroupInvitePayload struct {
    GroupID  string `json:"group_id"`
    FromUser string `json:"from_user"`
    ToUser   string `json:"to_user"`
}

type UserInfo struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Status   string `json:"status"`
}


type Error struct {
    Code    int
    Message string
}

func (e Error) Error() string {
    return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}



func NewError(code int, message string) Error {
    return Error{
        Code:    code,
        Message: message,
    }
}

func NewErrorMessage(code int, message string) Message {
    return NewMessage(TypeError, ErrorPayload{
        Code:    code,
        Message: message,
    })
}




func NewGroupCreate(name, description string, memberIDs []string) Message {
    return NewMessage(TypeGroupCreate, GroupCreatePayload{
        Name:        name,
        Description: description,
        MemberIDs:   memberIDs,
    })
}

func NewGroupJoin(groupID, userID string) Message {
    return NewMessage(TypeGroupJoin, GroupJoinPayload{
        GroupID: groupID,
        UserID:  userID,
    })
}


// Constantes de statut
const (
    StatusOnline  = "online"
    StatusAway    = "away"
    StatusOffline = "offline"
)

// Constantes de statut d'ami
const (
    FriendStatusPending  = "pending"
    FriendStatusAccepted = "accepted"
    FriendStatusRejected = "rejected"
)




type Message struct {
    Type      MessageType  `json:"type"`
    Payload   interface{} `json:"payload,omitempty"`
    Timestamp int64       `json:"timestamp"`
}

type AuthPayload struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type AuthResponsePayload struct {
    Success   bool   `json:"success"`
    UserID    string `json:"user_id"`
    Username  string `json:"username"`
    Token     string `json:"token,omitempty"`
    Error     string `json:"error,omitempty"`
}

type MessagePayload struct {
    ID        string `json:"id,omitempty"`
    Content   string `json:"content"`
    SenderID  string `json:"sender_id,omitempty"`
    Username  string `json:"username,omitempty"`
    GroupID   string `json:"group_id,omitempty"`
    Timestamp int64  `json:"timestamp,omitempty"`
    SenderName string `json:"sender_name,omitempty"`
}

func NewAuthResponse(success bool, userID, username string) Message {
    return Message{
        Type: TypeAuthResponse,
        Payload: AuthResponsePayload{
            Success:  success,
            UserID:   userID,
            Username: username,
        },
        Timestamp: time.Now().Unix(),
    }
}

func NewErrorResponse(err error) Message {
    return Message{
        Type: TypeError,
        Payload: map[string]interface{}{
            "error": err.Error(),
        },
        Timestamp: time.Now().Unix(),
    }
}

func NewMessage(msgType MessageType, payload interface{}) Message {
    return Message{
        Type:      msgType,
        Payload:   payload,
        Timestamp: time.Now().Unix(),
    }
}

// create a new message from a payload (global message)
func NewGlobalMessage(content string, senderID string, username string) Message {
    return Message{
        Type: TypeGlobalMessage,
        Payload: MessagePayload{
            Content:  content,
            SenderID: senderID,
            Username: username,
            SenderName: username,
        },
        Timestamp: time.Now().Unix(),
    }
}

// create a new message from a payload (direct message)
func NewDirectMessage(content string, senderID string, username string, recipientID string) Message {
    return Message{
        Type: TypeDirectMessage,
        Payload: map[string]interface{}{
            "content":      content,
            "sender_id":    senderID,
            "username":     username,
            "recipient_id": recipientID,
        },
        Timestamp: time.Now().Unix(),
    }
}

// create a new message from a payload (group message)
func NewGroupMessage(content string, senderID string, username string, groupID string) Message {
    return Message{
        Type: TypeGroupMessage,
        Payload: map[string]interface{}{
            "content":   content,
            "sender_id": senderID,
            "username":  username,
            "group_id":  groupID,
        },
        Timestamp: time.Now().Unix(),
    }
}

// create ping
func NewPingMessage() Message {
    return Message{
        Type:      TypePing,
        Timestamp: time.Now().Unix(),
    }
}

// create pong
func NewPongMessage() Message {
    return Message{
        Type:      TypePong,
        Timestamp: time.Now().Unix(),
    }
}

// create a new status update message
func NewStatusUpdate(userID string, status string) Message {
    return Message{
        Type: TypeStatusUpdate,
        Payload: map[string]interface{}{
            "user_id": userID,
            "status":  status,
        },
        Timestamp: time.Now().Unix(),
    }
}

type LoadMessagesPayload struct {
    BeforeID string `json:"before_id"`
    Limit    int    `json:"limit"`
}

func NewLoadMessagesRequest(beforeID string, limit int) Message {
    return Message{
        Type: TypeLoadMessages,
        Payload: LoadMessagesPayload{
            BeforeID: beforeID,
            Limit:    limit,
        },
        Timestamp: time.Now().Unix(),
    }
}

// FriendRequestSentMsg is sent to confirm that a friend request was sent
type FriendRequestSentMsg struct {
    FromUser  string `json:"from_user"`
    ToUser    string `json:"to_user"`
    RequestID string `json:"request_id"`
    Status    string `json:"status"`
}

// FriendRequestResponseMsg is sent when someone responds to a friend request
type FriendRequestResponseMsg struct {
    RequestID string `json:"request_id"`
    FromUser  string `json:"from_user"`
    ToUser    string `json:"to_user"`
    Accepted  bool   `json:"accepted"`
}

type FriendResponsePayload struct {
    RequestID string `json:"request_id"`
    FromUser  string `json:"from_user"`
    Accept    bool   `json:"accept"`
}