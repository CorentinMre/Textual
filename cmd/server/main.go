// cmd/server/main.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"textual/internal/server/database"
	"textual/internal/server/handlers"
	"textual/pkg/protocol"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Server struct {
    db           *database.DB
    clients      map[string]*handlers.Client
    mu           sync.RWMutex
    broadcast    chan protocol.Message
    authHandler  *handlers.AuthHandler
    msgHandler   *handlers.MessageHandler
}

func NewServer(db *database.DB) *Server {
    broadcast := make(chan protocol.Message, 100) // load 100 messages into the buffer
    clients := make(map[string]*handlers.Client)
    
    server := &Server{
        db:        db,
        clients:   clients,
        broadcast: broadcast,
    }

    server.authHandler = handlers.NewAuthHandler(db, clients, broadcast)
    server.msgHandler = handlers.NewMessageHandler(db, broadcast, clients)

    return server
}

func (s *Server) Start(port string) error {
    listener, err := net.Listen("tcp", ":"+port)
    if err != nil {
        return err
    }
    defer listener.Close()

    log.Printf("Server started on port %s", port)

    // start broadcast routine
    go s.handleBroadcast()

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Error accepting connection: %v", err)
            continue
        }

        go s.handleConnection(conn)
    }
}

func (s *Server) handleConnection(conn net.Conn) {
    defer func() {
        conn.Close()
        log.Printf("Connection closed")
    }()

    // authenticate user
    user, err := s.authHandler.HandleAuth(conn)
    if err != nil {
        log.Printf("Authentication error: %v", err)
        return
    }

    log.Printf("User %s authenticated successfully", user.Username)

    // new client
    client := handlers.NewClient(conn, user.ID, user.Username)

    // register client
    s.mu.Lock()
    s.clients[user.ID] = client
    s.mu.Unlock()

    log.Printf("Client registered: %s", user.Username)

    // disconnect client on exit
    defer func() {
        s.mu.Lock()
        if _, ok := s.clients[user.ID]; ok {
            log.Printf("Cleaning up client: %s", user.Username)
            client.Close()
            delete(s.clients, user.ID)
            s.authHandler.HandleLogout(user.ID)
        }
        s.mu.Unlock()
    }()

    // start clent routines
    errChan := make(chan error, 2)
    go s.readPump(client, errChan)
    go s.writePump(client, errChan)

    // wait for errors
    err = <-errChan
    if err != nil && err != io.EOF {
        log.Printf("Client error: %v", err)
    }
}

func (s *Server) readPump(client *handlers.Client, errChan chan<- error) {
    defer func() {
        errChan <- nil
    }()

    for {
        var msg protocol.Message
        decoder := json.NewDecoder(client.Conn)

        if err := decoder.Decode(&msg); err != nil {
            if err != io.EOF {
                errChan <- fmt.Errorf("read error: %v", err)
            }
            return
        }

        log.Printf("Received message from %s: %v", client.Username, msg.Type)

        // handle the type of message
        if err := s.msgHandler.HandleMessage(client.ID, msg); err != nil {
            log.Printf("Error handling message: %v", err)
            errorMsg := protocol.NewErrorMessage(protocol.ErrCodeInternalError, err.Error())
            select {
            case client.Send <- errorMsg:
            default:
                log.Printf("Client send channel full")
                errChan <- fmt.Errorf("client send channel full")
                return
            }
        }
    }
}

func (s *Server) writePump(client *handlers.Client, errChan chan<- error) {
    ticker := time.NewTicker(30 * time.Second)
    defer func() {
        ticker.Stop()
        errChan <- nil
    }()

    for {
        select {
        case msg, ok := <-client.Send:
            if !ok {
                errChan <- fmt.Errorf("client channel closed")
                return
            }

            client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            encoder := json.NewEncoder(client.Conn)
            if err := encoder.Encode(msg); err != nil {
                errChan <- fmt.Errorf("write error: %v", err)
                return
            }
            log.Printf("Sent message to %s: %v", client.Username, msg.Type)

        case <-ticker.C:
            client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            pingMsg := protocol.NewMessage(protocol.TypePing, nil)
            encoder := json.NewEncoder(client.Conn)
            if err := encoder.Encode(pingMsg); err != nil {
                errChan <- fmt.Errorf("ping error: %v", err)
                return
            }
        }
    }
}

func (s *Server) handleBroadcast() {
    for msg := range s.broadcast {
        s.mu.RLock()
        log.Printf("Broadcasting message type %v to %d clients", msg.Type, len(s.clients))
        for _, client := range s.clients {
            select {
            case client.Send <- msg:
                log.Printf("Broadcast message sent to %s", client.Username)
            default:
                log.Printf("Failed to send broadcast to %s: channel full", client.Username)
                client.Close()
                delete(s.clients, client.ID)
            }
        }
        s.mu.RUnlock()
    }
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatal("Error loading .env file")
    }

    db, err := database.NewDB(
        os.Getenv("DB_HOST"),
        os.Getenv("DB_PORT"),
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_NAME"),
    )
    if err != nil {
        log.Fatal("Database connection error:", err)
    }
    defer db.Close()

    server := NewServer(db)
    if err := server.Start(os.Getenv("SERVER_PORT")); err != nil {
        log.Fatal("Server error:", err)
    }
}
