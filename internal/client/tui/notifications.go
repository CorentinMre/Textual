// internal/client/tui/notifications.go
package tui

import (
	"fmt"
	"strings"
	"textual/internal/client/models"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NotificationView struct {
    viewport    viewport.Model
    notifications []models.Message
    width       int
    height      int
    style       lipgloss.Style
}

func NewNotificationView() *NotificationView {
    return &NotificationView{
        viewport:      viewport.New(0, 0),
        notifications: make([]models.Message, 0),
        style: lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            Padding(0, 1),
    }
}

func (n *NotificationView) Update(msg tea.Msg) tea.Cmd {
    var cmd tea.Cmd
    n.viewport, cmd = n.viewport.Update(msg)
    return cmd
}

func (n *NotificationView) View() string {
    return n.style.Render(n.viewport.View())
}

func (n *NotificationView) Resize(width, height int) {
    n.width = width
    n.height = height
    n.viewport.Width = width
    n.viewport.Height = height
    n.style = n.style.Width(width)
}

func (n *NotificationView) AddNotification(msg models.Message) {
    n.notifications = append(n.notifications, msg)
    n.updateContent()
}

func (n *NotificationView) updateContent() {
    var sb strings.Builder
    for _, notif := range n.notifications {
        sb.WriteString(fmt.Sprintf("New message from %s\n", notif.SenderID))
    }
    n.viewport.SetContent(sb.String())
}
