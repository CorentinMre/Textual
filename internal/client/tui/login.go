// internal/client/tui/login.go
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
    loginStyle = lipgloss.NewStyle().
        Align(lipgloss.Center).
        Border(lipgloss.RoundedBorder()).
        Padding(1, 2)

    titleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FF87D7")).
        Bold(true).
        MarginBottom(1)

    // errorStyle = lipgloss.NewStyle().
    //     Foreground(lipgloss.Color("#FF0000")).
    //     MarginTop(1)
)

type LoginModel struct {
    username    textinput.Model
    password    textinput.Model
    serverHost  textinput.Model
    serverPort  textinput.Model
    focusIndex  int
    err         error
    width       int
    height      int
}

func NewLoginModel() LoginModel {
    username := textinput.New()
    username.Placeholder = "Username"
    username.Focus()

    password := textinput.New()
    password.Placeholder = "Password"
    password.EchoMode = textinput.EchoPassword

    serverHost := textinput.New()
    serverHost.Placeholder = "Server Host (default: localhost)"

    serverPort := textinput.New()
    serverPort.Placeholder = "Server Port (default: 8080)"

    return LoginModel{
        username:    username,
        password:    password,
        serverHost:  serverHost,
        serverPort:  serverPort,
        focusIndex:  0,
    }
}

func (m LoginModel) Init() tea.Cmd {
    return textinput.Blink
}

func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            return m, tea.Quit
        case "tab", "shift+tab":
            // Cycle focus between all inputs
            if msg.String() == "tab" {
                m.focusIndex = (m.focusIndex + 1) % 4
            } else {
                m.focusIndex = (m.focusIndex - 1 + 4) % 4
            }

            for i := range []textinput.Model{m.username, m.password, m.serverHost, m.serverPort} {
                if i == m.focusIndex {
                    switch i {
                    case 0:
                        m.username.Focus()
                    case 1:
                        m.password.Focus()
                    case 2:
                        m.serverHost.Focus()
                    case 3:
                        m.serverPort.Focus()
                    }
                } else {
                    switch i {
                    case 0:
                        m.username.Blur()
                    case 1:
                        m.password.Blur()
                    case 2:
                        m.serverHost.Blur()
                    case 3:
                        m.serverPort.Blur()
                    }
                }
            }
            return m, nil

        case "enter":
            if m.username.Value() == "" || m.password.Value() == "" {
                m.err = fmt.Errorf("username and password are required")
                return m, nil
            }

            // Use default values if not specified
            host := "localhost"
            if m.serverHost.Value() != "" {
                host = m.serverHost.Value()
            }

            port := "8080"
            if m.serverPort.Value() != "" {
                port = m.serverPort.Value()
            }

            return m, func() tea.Msg {
                return LoginSuccessMsg{
                    Username:   m.username.Value(),
                    Password:   m.password.Value(),
                    ServerHost: host,
                    ServerPort: port,
                }
            }
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        
    case LoginErrorMsg:
        m.err = msg.Error
    }

    // Update all inputs
    var cmd tea.Cmd
    m.username, cmd = m.username.Update(msg)
    cmds = append(cmds, cmd)
    m.password, cmd = m.password.Update(msg)
    cmds = append(cmds, cmd)
    m.serverHost, cmd = m.serverHost.Update(msg)
    cmds = append(cmds, cmd)
    m.serverPort, cmd = m.serverPort.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}

func (m LoginModel) View() string {
    var content string

    // Title
    content += titleStyle.Render("Chat Application Login")
    content += "\n\n"

    // Inputs
    content += "Username:\n"
    content += m.username.View()
    content += "\n\nPassword:\n"
    content += m.password.View()
    content += "\n\nServer Host:\n"
    content += m.serverHost.View()
    content += "\n\nServer Port:\n"
    content += m.serverPort.View()
    content += "\n\n"

    // Help
    content += "Press Tab to switch fields • Enter to submit"

    // Error
    if m.err != nil {
        content += "\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
    }

    // Center everything
    return lipgloss.Place(
        m.width,
        m.height,
        lipgloss.Center,
        lipgloss.Center,
        loginStyle.Render(content),
    )
}

type LoginSuccessMsg struct {
    Username   string
    Password   string
    ServerHost string
    ServerPort string
}

type LoginErrorMsg struct {
    Error error
}
