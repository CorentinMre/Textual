// internal/client/tui/app.go
package tui

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"textual/internal/client/models"
	"textual/internal/client/network"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

type Page int

const (
	GlobalPage Page = iota
	GroupsPage
	MessagesPage
	FriendsPage
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#874BFD")).
			Padding(0, 1)

	tabStyle = headerStyle.
			Background(lipgloss.Color("#383838"))

	activeTabStyle = headerStyle.
			Background(lipgloss.Color("#874BFD")).
			Underline(true)

	// messageStyle = lipgloss.NewStyle().
	// 	PaddingLeft(2)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Width(10)

	// usernameStyle = lipgloss.NewStyle().
	// 	Bold(true).
	// 	Foreground(lipgloss.Color("#874BFD")).
	// 	PaddingRight(1)

	// contentStyle = lipgloss.NewStyle().
	// 	PaddingLeft(1)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
)

type Model struct {
	viewport        viewport.Model
	input           textinput.Model
	currentPage     Page
	messages        map[string][]models.Message
	selectedChat    string
	width           int
	height          int
	err             error
	onSendMessage   func(string, *string, *string) error
	onLoadMessages  func(string, int) error
	connection      *network.ConnectionHandler
	friendsView     *FriendsView
	userID          string
	isLoading       bool
	hasMoreMessages bool
}

type MessagesLoadedMsg struct {
	Messages []models.Message
}

func NewModel(onSendMessage func(string, *string, *string) error) Model {
    input := textinput.New()
    input.Placeholder = "Type a message..."
    input.Focus()
    input.CharLimit = 1000
 
    // get term size
    fd := uintptr(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
    if err != nil {
        width = 80  // Fallback
        height = 24
    }
 
    vp := viewport.New(width, height-4)
    vp.SetContent("")
 
    input.Width = width - 8
 
    return Model{
        viewport:        vp,
        input:          input,
        currentPage:    GlobalPage,
        messages:       make(map[string][]models.Message),
        selectedChat:   "global",
        onSendMessage:  onSendMessage,
        hasMoreMessages: true,
        width:          width,
        height:         height,
    }
 }

func (m *Model) SetConnectionHandler(handler *network.ConnectionHandler) {
	m.connection = handler
	// Réinitialiser la vue amis si elle existe
	if m.friendsView != nil {
		m.friendsView = NewFriendsView(handler)
		m.friendsView.onStartChat = func(friendID string) {
			m.currentPage = MessagesPage
			m.selectedChat = friendID
			m.updateContent()
		}
	}
}

func (m *Model) SetConnection(handler *network.ConnectionHandler) {
	m.connection = handler
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			oldPage := m.currentPage
			m.currentPage = (m.currentPage + 1) % 4
			// if m.currentPage == GlobalPage {
			//     m.selectedChat = "global"
			// }
			switch m.currentPage {
			case GlobalPage:
				m.selectedChat = "global"
				m.input.Focus()
				if m.friendsView != nil {
					m.friendsView.Blur()
				}

			case GroupsPage:
				m.input.Blur()
				// TODO: handle groups view

			case MessagesPage:
				if m.selectedChat != "" {
					m.input.Focus()
				} else {
					m.input.Blur()
				}

			case FriendsPage:
				m.input.Blur()
				if m.friendsView == nil && m.connection != nil {
					m.friendsView = NewFriendsView(m.connection)
					m.friendsView.onStartChat = func(friendID string) {
						m.currentPage = MessagesPage
						m.selectedChat = friendID
						m.input.Focus()
						m.updateContent()
					}
				}
				if m.friendsView != nil {
					m.friendsView.Focus()
				}
			}

			if oldPage == FriendsPage {
				if m.friendsView != nil {
					m.friendsView.Blur()
				}
			}

			m.updateContent()

		case "enter":
            if m.currentPage == FriendsPage && m.friendsView != nil {
                var cmd tea.Cmd
                m.friendsView.Update(msg)
                return m, cmd
            }

            // if m.currentPage == GroupsPage && m.groupsView != nil {
            //     var cmd tea.Cmd
            //     m.groupsView.Update(msg)
            //     return m, cmd
            // }

            if m.input.Value() != "" && m.onSendMessage != nil {
                content := m.input.Value()
                var err error

                switch m.currentPage {
                case GlobalPage:
                    err = m.onSendMessage(content, nil, nil)
                case MessagesPage:
                    if m.selectedChat != "" {
                        recipientID := m.selectedChat
                        err = m.onSendMessage(content, &recipientID, nil)
                    }
                }

                if err != nil {
                    m.err = err
                    log.Printf("Error sending message: %v", err)
                } else {
                    m.input.Reset()
                    m.viewport.GotoBottom()
                }
                return m, nil
            }

        default:
            if m.currentPage == FriendsPage && m.friendsView != nil {
                var cmd tea.Cmd
                m.friendsView.Update(msg)
                return m, cmd
            }

            // if m.currentPage == GroupsPage && m.groupsView != nil {
            //     var cmd tea.Cmd
            //     m.groupsView.Update(msg)
            //     return m, cmd
            // }
		}

	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp {
			if m.viewport.YOffset == 0 && !m.isLoading && m.hasMoreMessages {
				m.isLoading = true
				if m.onLoadMessages != nil && len(m.messages[m.selectedChat]) > 0 {
					firstMsg := m.messages[m.selectedChat][0]
					if err := m.onLoadMessages(firstMsg.ID, 50); err != nil {
						log.Printf("Failed to load more messages: %v", err)
						m.isLoading = false
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		inputHeight := 3
		verticalMargin := headerHeight + inputHeight + 1

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMargin
		m.input.Width = msg.Width - 8

		if m.friendsView != nil {
			m.friendsView.resize()
		}

		m.updateContent()

	case models.MessageReceived:
		log.Printf("Received message in TUI: %+v", msg.Message)
		chatID := m.getChatID(msg.Message)
		if m.messages[chatID] == nil {
			m.messages[chatID] = make([]models.Message, 0)
		}
		m.messages[chatID] = append(m.messages[chatID], msg.Message)

		if chatID == m.selectedChat {
			m.updateContent()
			m.viewport.GotoBottom()
		}

	case MessagesLoadedMsg:
		m.isLoading = false
		if len(msg.Messages) > 0 {
			chatID := m.selectedChat

			// slice for all messages
			allMessages := make([]models.Message, 0, len(msg.Messages)+len(m.messages[chatID]))

			// add older messages first
			allMessages = append(allMessages, msg.Messages...)

			// add newer messages
			allMessages = append(allMessages, m.messages[chatID]...)

			// update messages
			m.messages[chatID] = allMessages

			// update content
			m.updateContent()
		} else {
			m.hasMoreMessages = false
		}

		// Scroll to the last message
		if m.viewport.YOffset > 0 {
			m.viewport.SetYOffset(m.viewport.YOffset)
		}

	case models.ErrorMsg:
		m.err = fmt.Errorf("%s", msg.Error)
		log.Printf("Error received: %v", m.err)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Update input
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// Fonction helper pour calculer la hauteur des messages
// func (m Model) calculateMessagesHeight(messages []models.Message) int {
//     // Estimation simple : chaque message prend une ligne
//     // Ajustez selon votre mise en page réelle
//     return len(messages)
// }

func (m Model) View() string {
    var sb strings.Builder

    sb.WriteString(m.renderHeader())
    sb.WriteString("\n")

    if m.err != nil {
        sb.WriteString(errorStyle.Render(m.err.Error()))
        sb.WriteString("\n")
    }

    switch m.currentPage {
    case FriendsPage:
        if m.friendsView != nil {
            sb.WriteString(m.friendsView.View())
        }
    default:
        sb.WriteString(m.viewport.View())
        sb.WriteString("\n")
        sb.WriteString(inputStyle.Render(m.input.View()))
    }

    return sb.String()
}

func (m *Model) updateContent() {
    var content string
    switch m.currentPage {
    case GlobalPage:
        content = m.renderMessages(m.messages["global"])
    case GroupsPage:
        content = "group page (TODO)"
    case MessagesPage:
        // if m.selectedChat != "" {
        //     content = m.renderMessages(m.messages[m.selectedChat])
        // } else {
        //     content = "Select a friend to start a conversation"
        // }
		content = "private message (TODO)"
    case FriendsPage:
        if m.friendsView != nil {
            content = m.friendsView.View()
        }
    }

    m.viewport.SetContent(content)
    if m.currentPage == GlobalPage || m.currentPage == MessagesPage {
        m.viewport.GotoBottom()
    }
}

var (
	timestampStyleBase = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666"))

	usernameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#874BFD")).
			PaddingRight(1).
			Width(15).
			Align(lipgloss.Left)

	contentStyle = lipgloss.NewStyle().
			PaddingLeft(1)
)

func (m Model) renderMessages(messages []models.Message) string {
	var sb strings.Builder

	if m.isLoading {
		sb.WriteString("Loading more messages...\n")
	}

	sortedMessages := make([]models.Message, len(messages))
	copy(sortedMessages, messages)

	sort.Slice(sortedMessages, func(i, j int) bool {
		if sortedMessages[i].SentAt.Equal(sortedMessages[j].SentAt) {
			return sortedMessages[i].ID < sortedMessages[j].ID
		}
		return sortedMessages[i].SentAt.Before(sortedMessages[j].SentAt)
	})

	for _, msg := range sortedMessages {
		timestamp := m.formatTimestamp(msg.SentAt.Local()) // convert to local time
		senderName := msg.SenderName

		timestampStyle := timestampStyleBase
		if len(timestamp) > 8 {
			timestampStyle = timestampStyle.Width(20)
		} else {
			timestampStyle = timestampStyle.Width(10)
		}

		timeStr := timestampStyle.Render(timestamp)
		nameStr := usernameStyle.Render(senderName)
		contentStr := contentStyle.Render(msg.Content)

		line := fmt.Sprintf("%s%s%s\n", timeStr, nameStr, contentStr)
		sb.WriteString(line)
	}
	return sb.String()
}

func (m Model) formatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day() {
		return t.Format("15:04:05")
	} else if t.Year() == now.Year() {
		return fmt.Sprintf("[%s %s]", t.Format("02/01"), t.Format("15:04:05"))
	}
	return fmt.Sprintf("[%s %s]", t.Format("02/01/06"), t.Format("15:04:05"))
}

func (m Model) renderHeader() string {
    tabNames := []string{
        "Global",
        "Groups",
        "Messages",
        "Friends",
    }

    var renderedTabs []string
    for i, name := range tabNames {
        style := tabStyle
        if Page(i) == m.currentPage {
            style = activeTabStyle
        }
        
        // count pending friend requests
        if Page(i) == FriendsPage && m.friendsView != nil && len(m.friendsView.pendingRequests) > 0 {
            name = fmt.Sprintf("%s +%d", name, len(m.friendsView.pendingRequests))
        }
        
        renderedTabs = append(renderedTabs, style.Render(name))
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}

func (m Model) getChatID(msg models.Message) string {
	if msg.GroupID != nil {
		return *msg.GroupID
	}
	if msg.RecipientID != nil {
		return *msg.RecipientID
	}
	return "global"
}

func (m *Model) SetUserID(userID string) {
	m.userID = userID
}

func (m *Model) AddMessage(msg models.Message) {
	chatID := m.getChatID(msg)
	if m.messages[chatID] == nil {
		m.messages[chatID] = make([]models.Message, 0)
	}
	m.messages[chatID] = append(m.messages[chatID], msg)

	if chatID == m.selectedChat {
		m.updateContent()
		m.viewport.GotoBottom()
	}
}
