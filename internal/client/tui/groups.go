// internal/client/tui/groups.go
package tui

import (
	"fmt"
	"strings"
	"textual/internal/client/models"
	"textual/internal/client/network"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GroupMode represents the different states of the group view
type GroupMode int

const (
    GroupListMode GroupMode = iota
    GroupChatMode
    GroupCreateMode
)

type GroupsView struct {
    viewport         viewport.Model
    input           textinput.Model
    nameInput       textinput.Model
    descInput       textinput.Model
    messages        map[string][]models.Message
    selectedGroup   string
    width           int
    height          int
    style           lipgloss.Style
    onSendMessage   func(string, *string, *string) error
    list            list.Model
    groups          []models.Group
    mode            GroupMode
    connection      *network.ConnectionHandler
    userID          string
    focused         bool
    activeInput     int // 0: list, 1: input
    error           string
    loading         bool
}

func NewGroupsView(onSendMessage func(string, *string, *string) error, connection *network.ConnectionHandler) *GroupsView {
    input := textinput.New()
    input.Placeholder = "Type a message..."
    input.CharLimit = 500

    nameInput := textinput.New()
    nameInput.Placeholder = "Group name"
    nameInput.CharLimit = 50

    descInput := textinput.New()
    descInput.Placeholder = "Group description"
    descInput.CharLimit = 200

    delegate := list.NewDefaultDelegate()
    delegate.ShowDescription = true

    l := list.New(nil, delegate, 0, 0)
    l.Title = "Groups"
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(false)
    l.Styles.Title = titleStyle

    return &GroupsView{
        viewport:       viewport.New(0, 0),
        input:         input,
        nameInput:     nameInput,
        descInput:     descInput,
        messages:      make(map[string][]models.Message),
        style:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1),
        onSendMessage: onSendMessage,
        list:          l,
        groups:        make([]models.Group, 0),
        mode:          GroupListMode,
        connection:    connection,
        focused:       false,
        activeInput:   0,
    }
}

func (g *GroupsView) SetUserID(userID string) {
    g.userID = userID
}

func (g *GroupsView) Update(msg tea.Msg) tea.Cmd {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+n":
            if g.mode == GroupListMode {
                g.mode = GroupCreateMode
                g.nameInput.Focus()
                g.activeInput = 0
                return nil
            }

        case "esc":
            switch g.mode {
            case GroupChatMode:
                g.mode = GroupListMode
                g.selectedGroup = ""
                g.input.Reset()
                g.updateGroupList()
            case GroupCreateMode:
                g.mode = GroupListMode
                g.nameInput.Reset()
                g.descInput.Reset()
            }
            return nil

        case "tab":
            switch g.mode {
            case GroupCreateMode:
                if g.activeInput == 0 {
                    g.nameInput.Blur()
                    g.descInput.Focus()
                    g.activeInput = 1
                } else {
                    g.nameInput.Focus()
                    g.descInput.Blur()
                    g.activeInput = 0
                }
                return nil
            }

        case "enter":
            switch g.mode {
            case GroupListMode:
                if item, ok := g.list.SelectedItem().(groupItem); ok {
                    g.selectedGroup = item.group.ID
                    g.mode = GroupChatMode
                    g.input.Focus()
                    g.updateContent()
                }
                return nil

            case GroupChatMode:
                if g.input.Value() != "" {
                    content := g.input.Value()
                    if g.onSendMessage != nil {
                        if err := g.onSendMessage(content, nil, &g.selectedGroup); err != nil {
                            g.error = fmt.Sprintf("Error sending message: %v", err)
                        } else {
                            g.input.Reset()
                            g.viewport.GotoBottom()
                        }
                    }
                }
                return nil

            case GroupCreateMode:
                if g.nameInput.Value() != "" {
                    name := g.nameInput.Value()
                    desc := g.descInput.Value()
                    if err := g.connection.CreateGroup(name, desc); err != nil {
                        g.error = fmt.Sprintf("Error creating group: %v", err)
                    } else {
                        g.mode = GroupListMode
                        g.nameInput.Reset()
                        g.descInput.Reset()
                        g.loading = true
                        // Group will be added when server confirms creation
                    }
                }
                return nil
            }
        }

        // Handle input updates based on mode
        switch g.mode {
        case GroupChatMode:
            if g.input.Focused() {
                var cmd tea.Cmd
                g.input, cmd = g.input.Update(msg)
                return cmd
            }
        case GroupCreateMode:
            if g.activeInput == 0 {
                var cmd tea.Cmd
                g.nameInput, cmd = g.nameInput.Update(msg)
                return cmd
            } else {
                var cmd tea.Cmd
                g.descInput, cmd = g.descInput.Update(msg)
                return cmd
            }
        case GroupListMode:
            var cmd tea.Cmd
            g.list, cmd = g.list.Update(msg)
            return cmd
        }

    case models.MessageReceived:
        if msg.Message.GroupID != nil {
            g.AddMessage(msg.Message)
            if g.mode == GroupChatMode && *msg.Message.GroupID == g.selectedGroup {
                g.viewport.GotoBottom()
            }
        }
        return nil
    }

    return tea.Batch(cmds...)
}

func (g *GroupsView) View() string {
    var sb strings.Builder

    // Show error if any
    if g.error != "" {
        sb.WriteString(errorStyle.Render(g.error))
        sb.WriteString("\n\n")
    }

    switch g.mode {
    case GroupListMode:
        if g.loading {
            sb.WriteString("Loading groups...\n")
        } else {
            sb.WriteString(g.list.View())
            sb.WriteString("\n\nPress Ctrl+N to create a new group")
        }

    case GroupChatMode:
        if messages, ok := g.messages[g.selectedGroup]; ok {
            for _, msg := range messages {
                timestamp := msg.SentAt.Format("15:04:05")
                senderName := msg.SenderName
                if msg.SenderID == g.userID {
                    senderName = "You"
                }
                
                line := fmt.Sprintf("%s %s: %s\n",
                    timestampStyle.Render(timestamp),
                    usernameStyle.Render(senderName),
                    contentStyle.Render(msg.Content))
                sb.WriteString(line)
            }
        }
        sb.WriteString("\n")
        sb.WriteString(g.input.View())

    case GroupCreateMode:
        sb.WriteString("Create New Group\n\n")
        sb.WriteString("Name:\n")
        sb.WriteString(g.nameInput.View())
        sb.WriteString("\n\nDescription:\n")
        sb.WriteString(g.descInput.View())
        sb.WriteString("\n\nPress Enter to create, Esc to cancel")
    }

    return g.style.Render(sb.String())
}

func (g *GroupsView) AddMessage(msg models.Message) {
    if msg.GroupID == nil {
        return
    }
    
    groupID := *msg.GroupID
    if g.messages[groupID] == nil {
        g.messages[groupID] = make([]models.Message, 0)
    }
    
    g.messages[groupID] = append(g.messages[groupID], msg)
    
    if groupID == g.selectedGroup {
        g.updateContent()
    }
}

func (g *GroupsView) updateContent() {
    if g.selectedGroup == "" {
        return
    }

    var content strings.Builder
    for _, msg := range g.messages[g.selectedGroup] {
        timestamp := msg.SentAt.Format("15:04:05")
        sender := msg.SenderName
        if msg.SenderID == g.userID {
            sender = "You"
        }
        content.WriteString(fmt.Sprintf("%s %s: %s\n",
            timestamp,
            sender,
            msg.Content))
    }
    
    g.viewport.SetContent(content.String())
    g.viewport.GotoBottom()
}

func (g *GroupsView) SetGroups(groups []models.Group) {
    g.groups = groups
    g.loading = false
    g.updateGroupList()
}

func (g *GroupsView) updateGroupList() {
    var items []list.Item
    for _, group := range g.groups {
        var lastMsg string
        var unreadCount int
        
        if messages, ok := g.messages[group.ID]; ok && len(messages) > 0 {
            lastMsg = messages[len(messages)-1].Content
            for _, msg := range messages {
                if !msg.Read && msg.SenderID != g.userID {
                    unreadCount++
                }
            }
        }

        items = append(items, groupItem{
            group:       group,
            lastMsg:     lastMsg,
            unreadCount: unreadCount,
        })
    }

    g.list.SetItems(items)
}

type groupItem struct {
    group       models.Group
    lastMsg     string
    unreadCount int
}

func (i groupItem) Title() string {
    if i.unreadCount > 0 {
        return fmt.Sprintf("📦 %s (%d unread)", i.group.Name, i.unreadCount)
    }
    return fmt.Sprintf("📦 %s", i.group.Name)
}

func (i groupItem) Description() string {
    if i.lastMsg != "" {
        return fmt.Sprintf("Last message: %s - %d members", i.lastMsg, len(i.group.Members))
    }
    return fmt.Sprintf("%d members", len(i.group.Members))
}

func (i groupItem) FilterValue() string {
    return i.group.Name
}

func (g *GroupsView) Resize(width, height int) {
    g.width = width
    g.height = height
    g.list.SetSize(width, height)
    g.viewport.Width = width
    g.viewport.Height = height - 3 // space for input
    g.input.Width = width - 4
    g.style = g.style.Width(width)
}

func (g *GroupsView) Focus() {
    g.focused = true
    switch g.mode {
    case GroupChatMode:
        g.input.Focus()
    case GroupCreateMode:
        if g.activeInput == 0 {
            g.nameInput.Focus()
        } else {
            g.descInput.Focus()
        }
    }
}

func (g *GroupsView) Blur() {
    g.focused = false
    g.input.Blur()
    g.nameInput.Blur()
    g.descInput.Blur()
}