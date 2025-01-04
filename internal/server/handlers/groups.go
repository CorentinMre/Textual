// internal/server/handlers/groups.go
package handlers

import (
	"textual/internal/server/database"
	"textual/internal/server/models"
	"textual/pkg/protocol"
)

type GroupHandler struct {
    db        *database.DB
    broadcast chan<- protocol.Message
}

func NewGroupHandler(db *database.DB, broadcast chan<- protocol.Message) *GroupHandler {
    return &GroupHandler{
        db:        db,
        broadcast: broadcast,
    }
}


func (h *GroupHandler) HandleGroupCreate(userID string, payload protocol.GroupCreatePayload) error {
    // create the group
    group, err := h.db.CreateGroup(
        payload.Name,
        payload.Description,
        userID,
    )
    if err != nil {
        return err
    }

    // add the creator to the group
    for _, memberID := range payload.MemberIDs {
        if err := h.db.AddUserToGroup(memberID, group.ID); err != nil {
            continue // Log error but continue
        }

        // Notify the group members
        h.broadcast <- protocol.NewMessage(protocol.TypeGroupJoin, protocol.GroupJoinPayload{
            GroupID: group.ID,
            UserID:  memberID,
        })
    }

    // Notify the creator
    h.broadcast <- protocol.NewMessage(protocol.TypeGroupCreate, group)
    return nil
}


func (h *GroupHandler) HandleGroupJoin(userID string, groupID string) error {
    // check if the group exists
    group, err := h.db.GetGroup(groupID)
    if err != nil {
        return err
    }

    // add the user to the group
    if err := h.db.AddUserToGroup(userID, group.ID); err != nil {
        return err
    }

    // Notify the group members
    h.broadcast <- protocol.NewMessage(protocol.TypeGroupJoin, protocol.GroupJoinPayload{
        GroupID: group.ID,
        UserID:  userID,
    })

    return nil
}


func (h *GroupHandler) HandleGroupMessage(userID string, payload protocol.GroupMessagePayload) error {
    // check if the user is a member of the group
    isMember, err := h.db.IsGroupMember(userID, payload.GroupID)
    if err != nil || !isMember {
        return protocol.NewError(protocol.ErrCodeNotAuthorized, "Not a member of this group")
    }

    // save the message
    msg := &models.Message{
        SenderID: userID,
        GroupID:  &payload.GroupID,
        Content:  payload.Content,
    }
    
    if err := h.db.SaveMessage(msg); err != nil {
        return err
    }

    // broadcast the message
    h.broadcast <- protocol.NewMessage(protocol.TypeGroupMessage, msg)
    return nil
}

func (h *GroupHandler) HandleGroupLeave(userID string, groupID string) error {
    // check if the user is a member of the group
    isMember, err := h.db.IsGroupMember(userID, groupID)
    if err != nil {
        return err
    }
    if !isMember {
        return protocol.NewError(protocol.ErrCodeNotAuthorized, "Not a member of this group")
    }

    // remove the user from the group
    if err := h.db.RemoveUserFromGroup(userID, groupID); err != nil {
        return err
    }

    // Notify the group members
    h.broadcast <- protocol.NewMessage(protocol.TypeGroupLeave, protocol.GroupJoinPayload{
        GroupID: groupID,
        UserID:  userID,
    })

    return nil
}

func (h *GroupHandler) GetGroupMessages(groupID string) ([]models.Message, error) {
    return h.db.GetGroupMessages(groupID)
}
