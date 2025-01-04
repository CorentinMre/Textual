// internal/client/tui/friends.go
package tui

import (
	"fmt"
	"log"
	"strings"
	"textual/internal/client/models"
	"textual/internal/client/network"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
    friendsViewStyle = lipgloss.NewStyle().
        Padding(1, 2)

    friendTitleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FF87D7"))

    // friendInputStyle = lipgloss.NewStyle().
    //     BorderStyle(lipgloss.RoundedBorder()).
    //     BorderForeground(lipgloss.Color("#874BFD")).
    //     Padding(0, 1)
    
    successStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#5AF78E"))

    notificationStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFD700")).
        Bold(true)

    pendingRequestStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFB6C1")).
        Bold(true)
)

type Notification struct {
    Message   string
    Timestamp time.Time
    IsError   bool
}

type FriendsView struct {
    list              list.Model
    searchInput       textinput.Model
    friends           []models.User
    pendingRequests   []models.FriendRequest
    sentRequests      []models.FriendRequest
    width             int
    height            int
    connectionHandler *network.ConnectionHandler
    onStartChat       func(string)
    notifications     []Notification
    userID           string
}

func NewFriendsView(handler *network.ConnectionHandler) *FriendsView {
    searchInput := textinput.New()
    searchInput.Placeholder = "Search for a user..."
    searchInput.Focus()
    searchInput.CharLimit = 100

    delegate := list.NewDefaultDelegate()
    delegate.ShowDescription = true

    l := list.New([]list.Item{}, delegate, 0, 0)
    l.Title = "Friends"
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(false)

    return &FriendsView{
        list:              l,
        searchInput:       searchInput,
        friends:           make([]models.User, 0),
        pendingRequests:   make([]models.FriendRequest, 0),
        sentRequests:      make([]models.FriendRequest, 0),
        connectionHandler: handler,
        notifications:     make([]Notification, 0),
    }
}

func (f *FriendsView) Init() tea.Cmd {
    return textinput.Blink
}

func (f *FriendsView) SetUserID(userID string) {
    f.userID = userID
}

func (f *FriendsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "y":
            if item, ok := f.list.SelectedItem().(requestItem); ok && !item.isSent {
                err := f.AcceptRequest(item.request.ID)
                if err != nil {
                    f.addNotification(fmt.Sprintf("Error: %v", err), true)
                } else {
                    f.addNotification(fmt.Sprintf("Accepted friend request from %s", item.request.FromUser), false)
                    f.RemovePendingRequest(item.request.ID)
                }
            }

        case "enter":
            if f.searchInput.Value() != "" {
                username := f.searchInput.Value()
                if err := f.AddFriend(username); err != nil {
                    f.addNotification(fmt.Sprintf("Error: %v", err), true)
                }
                f.searchInput.Reset()
                f.updateItems()
                return f, tea.Batch(cmds...)
            }
        }

        var cmd tea.Cmd
        f.searchInput, cmd = f.searchInput.Update(msg)
        cmds = append(cmds, cmd)
        return f, tea.Batch(cmds...)

    case models.MessageReceived:
        log.Printf("Message received in FriendsView: %+v", msg.Message)
        if msg.Message.Content == "Friend request" {
            log.Printf("Processing friend request from %s", msg.Message.SenderName)
            newRequest := models.FriendRequest{
                ID:        msg.Message.ID,
                FromUser:  msg.Message.SenderName,
                CreatedAt: msg.Message.SentAt,
                Status:    "pending",
            }
            f.pendingRequests = append(f.pendingRequests, newRequest)
            f.addNotification(fmt.Sprintf("New friend request from %s", msg.Message.SenderName), false)
            f.updateItems()
        }
        return f, nil

    case tea.WindowSizeMsg:
        f.width = msg.Width
        f.height = msg.Height
        f.resize()
        f.updateItems()
        return f, nil
    }

    return f, tea.Batch(cmds...)
}

func (f *FriendsView) View() string {
    var sb strings.Builder

    // search input
    sb.WriteString("Search for users (press Enter to send friend request):\n")
    sb.WriteString(f.searchInput.View())
    sb.WriteString("\n\n")

    // Display pending requests
    if len(f.pendingRequests) > 0 {
        sb.WriteString(pendingRequestStyle.Render("🔔 Pending Friend Requests:"))
        sb.WriteString("\n")
        for _, req := range f.pendingRequests {
            sb.WriteString(fmt.Sprintf("📨 From %s - [y] Accept • [n] Reject\n", req.FromUser))
        }
        sb.WriteString("\n")
    }

    // Display sent requests
    if len(f.sentRequests) > 0 {
        sb.WriteString(pendingRequestStyle.Render("📤 Sent Friend Requests:"))
        sb.WriteString("\n")
        for _, req := range f.sentRequests {
            sb.WriteString(fmt.Sprintf("📤 To %s - Pending...\n", req.ToUser))
        }
        sb.WriteString("\n")
    }

    // Display notifications
    if len(f.notifications) > 0 {
        sb.WriteString(notificationStyle.Render("Recent Activity:"))
        sb.WriteString("\n")
        start := len(f.notifications) - 5
        if start < 0 {
            start = 0
        }
        for _, notif := range f.notifications[start:] {
            style := successStyle
            if notif.IsError {
                style = errorStyle
            }
            sb.WriteString(style.Render(fmt.Sprintf("[%s] %s\n",
                notif.Timestamp.Format("15:04:05"),
                notif.Message)))
        }
        sb.WriteString("\n")
    }

    // Display friends
    if len(f.friends) > 0 {
        sb.WriteString(friendTitleStyle.Render("Friends"))
        sb.WriteString("\n")
        for _, friend := range f.friends {
            statusIcon := "⭘"
            if friend.Status == "online" {
                statusIcon = "🟢"
            } else if friend.Status == "away" {
                statusIcon = "🟡"
            }
            sb.WriteString(fmt.Sprintf("%s %s\n", statusIcon, friend.Username))
        }
    }

    return friendsViewStyle.Render(sb.String())
}

func (f *FriendsView) AddFriend(username string) error {
    if f.connectionHandler == nil {
        return fmt.Errorf("not connected")
    }

    // Add to sent requests for immediate UI feedback
    newRequest := models.FriendRequest{
        FromUser: "You",
        ToUser:   username,
        Status:   "pending",
    }
    f.sentRequests = append(f.sentRequests, newRequest)
    f.updateItems()
    
    // Send the actual request
    if err := f.connectionHandler.SendFriendRequest(username); err != nil {
        // Remove from sent requests if failed
        f.removeSentRequest(username)
        return err
    }

    f.addNotification(fmt.Sprintf("Friend request sent to %s", username), false)
    return nil
}

func (f *FriendsView) removeSentRequest(username string) {
    for i, req := range f.sentRequests {
        if req.ToUser == username {
            f.sentRequests = append(f.sentRequests[:i], f.sentRequests[i+1:]...)
            break
        }
    }
    f.updateItems()
}

func (f *FriendsView) RemovePendingRequest(requestID string) {
    for i, req := range f.pendingRequests {
        if req.ID == requestID {
            f.pendingRequests = append(f.pendingRequests[:i], f.pendingRequests[i+1:]...)
            break
        }
    }
    f.updateItems()
}

func (f *FriendsView) AcceptRequest(requestID string) error {
    if f.connectionHandler == nil {
        return fmt.Errorf("not connected")
    }

    if err := f.connectionHandler.AcceptFriendRequest(requestID); err != nil {
        return err
    }

    return nil
}

func (f *FriendsView) addNotification(msg string, isError bool) {
    notification := Notification{
        Message:   msg,
        Timestamp: time.Now(),
        IsError:   isError,
    }
    f.notifications = append(f.notifications, notification)
    
    // Keep only last 10 notifications
    if len(f.notifications) > 10 {
        f.notifications = f.notifications[len(f.notifications)-10:]
    }
}

func (f *FriendsView) updateItems() {
    var items []list.Item

    // Add pending requests
    for _, req := range f.pendingRequests {
        items = append(items, requestItem{request: req, isSent: false})
    }

    // Add sent requests
    for _, req := range f.sentRequests {
        items = append(items, requestItem{request: req, isSent: true})
    }

    // Add friends
    for _, friend := range f.friends {
        items = append(items, friendItem{user: friend})
    }

    f.list.SetItems(items)
}

func (f *FriendsView) resize() {
    f.list.SetSize(f.width-4, f.height-6)
}

func (f *FriendsView) Focus() {
    f.searchInput.Focus()
}

func (f *FriendsView) Blur() {
    f.searchInput.Blur()
}

// Types for list items
type friendItem struct {
    user models.User
}

func (i friendItem) Title() string {
    statusIcon := "⭘" // offline
    if i.user.Status == "online" {
        statusIcon = "🟢"
    } else if i.user.Status == "away" {
        statusIcon = "🟡"
    }
    return fmt.Sprintf("%s %s", statusIcon, i.user.Username)
}

func (i friendItem) Description() string {
    return fmt.Sprintf("Status: %s", i.user.Status)
}

func (i friendItem) FilterValue() string {
    return i.user.Username
}

type requestItem struct {
    request models.FriendRequest
    isSent  bool
}

func (i requestItem) Title() string {
    if i.isSent {
        return fmt.Sprintf("📤 Request sent to %s", i.request.ToUser)
    }
    return fmt.Sprintf("📨 Request from %s", i.request.FromUser)
}

func (i requestItem) Description() string {
    if i.isSent {
        return "Waiting for response..."
    }
    return "Press [y] to accept or [n] to reject"
}

func (i requestItem) FilterValue() string {
    if i.isSent {
        return i.request.ToUser
    }
    return i.request.FromUser
}
