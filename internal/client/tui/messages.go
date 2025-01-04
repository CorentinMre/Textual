// internal/client/tui/messages.go
package tui

import (
	"textual/internal/client/models"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

type MessagesView struct {
    viewport    viewport.Model
    input       textinput.Model
    messages    map[string][]models.Message
    activeChat  *string
}

func NewMessagesView() *MessagesView {
    return &MessagesView{
        viewport:   viewport.New(0, 0),
        input:      textinput.New(),
        messages:   make(map[string][]models.Message),
        activeChat: nil,
    }
}

func (m *MessagesView) SetActiveChat(id string) {
    m.activeChat = &id
}

