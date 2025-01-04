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
    focusIndex int
    err        error
    width      int
    height     int
}

func NewLoginModel() LoginModel {
    username := textinput.New()
    username.Placeholder = "Username"
    username.Focus()

    password := textinput.New()
    password.Placeholder = "Password"
    password.EchoMode = textinput.EchoPassword

    return LoginModel{
        username:    username,
        password:    password,
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
            // Cycle focus
            if msg.String() == "tab" {
                m.focusIndex = (m.focusIndex + 1) % 2
            } else {
                m.focusIndex = (m.focusIndex - 1 + 2) % 2
            }

            if m.focusIndex == 0 {
                m.username.Focus()
                m.password.Blur()
            } else {
                m.username.Blur()
                m.password.Focus()
            }

            return m, nil

        case "enter":
            if m.username.Value() == "" || m.password.Value() == "" {
                m.err = fmt.Errorf("username and password are required")
                return m, nil
            }
            // send login message
            return m, func() tea.Msg {
                return LoginSuccessMsg{
                    Username: m.username.Value(),
                    Password: m.password.Value(),
                }
            }
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }

    // Update the inputs
    var cmd tea.Cmd
    m.username, cmd = m.username.Update(msg)
    cmds = append(cmds, cmd)
    m.password, cmd = m.password.Update(msg)
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
    content += "\n\n"

    // Help
    content += "Press Tab to switch fields • Enter to submit"

    // Error
    if m.err != nil {
        content += "\n" + errorStyle.Render(m.err.Error())
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

// Messages pour la communication entre les modèles
type LoginSuccessMsg struct {
    Username string
    Password string
}