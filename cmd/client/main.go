// cmd/client/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"textual/internal/client/models"
	"textual/internal/client/network"
	"textual/internal/client/tui"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)


type AppModel struct {
    loginModel  tui.LoginModel
    chatModel   tui.Model
    connection  *network.ConnectionHandler
    isLoggedIn bool
    err        error
}


func NewAppModel() AppModel {
    return AppModel{
        loginModel: tui.NewLoginModel(),
        chatModel:  tui.NewModel(nil),
    }
}


func (m AppModel) Init() tea.Cmd {
    return m.loginModel.Init()
}

// state machine
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case tui.LoginSuccessMsg:
        var err error
        serverAddr := fmt.Sprintf("%s:%s", msg.ServerHost, msg.ServerPort)
        m.connection, err = m.setupConnection(msg.Username, msg.Password, serverAddr)
        if err != nil {
            log.Printf("Setup connection error: %v", err)
            newModel, newCmd := m.loginModel.Update(tui.LoginErrorMsg{Error: err})
            if loginModel, ok := newModel.(tui.LoginModel); ok {
                m.loginModel = loginModel
                return m, newCmd
            }
            return m, nil
        }
        m.isLoggedIn = true

        // conf of callback to send messages
        sendMessage := func(content string, recipientID *string, groupID *string) error {
            return m.connection.SendMessage(content, recipientID, groupID)
        }

        // init chat model
        m.chatModel = tui.NewModel(sendMessage)
        m.chatModel.SetConnection(m.connection)

        return m, nil

    case models.MessageReceived:
        if m.isLoggedIn {
            newModel, newCmd := m.chatModel.Update(msg)
            if chatModel, ok := newModel.(tui.Model); ok {
                m.chatModel = chatModel
                cmd = newCmd
            }
        }

    case models.FriendRequestReceived:
        if m.isLoggedIn {
            newModel, newCmd := m.chatModel.Update(msg)
            if chatModel, ok := newModel.(tui.Model); ok {
                m.chatModel = chatModel
                cmd = newCmd
            }
        }

    case models.StatusUpdate:
        if m.isLoggedIn {
            newModel, newCmd := m.chatModel.Update(msg)
            if chatModel, ok := newModel.(tui.Model); ok {
                m.chatModel = chatModel
                cmd = newCmd
            }
        }

    case models.ErrorMsg:
        if !m.isLoggedIn {
            // if not logged in, show error in login screen
            newModel, newCmd := m.loginModel.Update(tui.LoginErrorMsg{Error: fmt.Errorf(msg.Error)})
            if loginModel, ok := newModel.(tui.LoginModel); ok {
                m.loginModel = loginModel
                return m, newCmd
            }
        } else {
            // else show in chat screen
            newModel, newCmd := m.chatModel.Update(msg)
            if chatModel, ok := newModel.(tui.Model); ok {
                m.chatModel = chatModel
                cmd = newCmd
            }
        }

    default:
        if m.isLoggedIn {
            newModel, newCmd := m.chatModel.Update(msg)
            if chatModel, ok := newModel.(tui.Model); ok {
                m.chatModel = chatModel
                cmd = newCmd
            }
        } else {
            newModel, newCmd := m.loginModel.Update(msg)
            if loginModel, ok := newModel.(tui.LoginModel); ok {
                m.loginModel = loginModel
                cmd = newCmd
            }
        }
    }

    return m, cmd
}

// return the view of the current state
func (m AppModel) View() string {
    if m.err != nil {
        return fmt.Sprintf("Error: %v", m.err)
    }

    if m.isLoggedIn {
        return m.chatModel.View()
    }
    return m.loginModel.View()
}

// setup connection with serv
func (m *AppModel) setupConnection(username, password, serverAddr string) (*network.ConnectionHandler, error) {
    log.Printf("Setting up connection for user: %s to server: %s", username, serverAddr)
    
    
    conn, err := network.NewConnection(serverAddr)
    if err != nil {
        return nil, fmt.Errorf("connection error: %v", err)
    }

    
    handler := network.NewConnectionHandler(conn.GetUnderlyingConn())
    
    
    handler.SetErrorHandler(func(err error) {
        log.Printf("Error received: %v", err)
        if p != nil {
            p.Send(models.ErrorMsg{Error: err.Error()})
        }
    })

    // temp conf
    handler.SetMessageHandler(func(msg models.Message) {
        log.Printf("Message received in main: %+v", msg)
        if p != nil {
            if msg.Content == "Friend request" {
                p.Send(models.FriendRequestReceived{
                    Request: models.FriendRequest{
                        ID:        msg.ID,
                        FromUser:  msg.SenderID,
                        ToUser:    "",
                        Status:    "pending",
                        CreatedAt: msg.SentAt,
                    },
                })
            } else {
                p.Send(models.MessageReceived{Message: msg})
            }
        }
    })

    // start the handler
    handler.Start()

    // try to authenticate
    if err := handler.SendAuthRequest(username, password); err != nil {
        return nil, fmt.Errorf("authentication error: %v", err)
    }

    // wait for authentication
    startTime := time.Now()
    for !handler.IsAuthenticated() {
        if time.Since(startTime) > 5*time.Second {
            return nil, fmt.Errorf("authentication timeout")
        }
        // check for auth error
        if handler.GetAuthError() != nil {
            return nil, handler.GetAuthError()
        }
        time.Sleep(100 * time.Millisecond)
    }

    log.Printf("Connection setup complete, authenticated: %v", handler.IsAuthenticated())
    return handler, nil
}

var p *tea.Program

func main() {
    // log file
    logFile, err := os.OpenFile("client.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal("Error opening log file:", err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)

    // init app model
    model := NewAppModel()

    // start program
    p = tea.NewProgram(model, tea.WithAltScreen())
    
    if err := p.Start(); err != nil {
        log.Fatal("Error running program:", err)
    }
}
