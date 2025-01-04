// internal/server/handlers/client.go
package handlers

import (
	"net"
	"textual/pkg/protocol"
)

type Client struct {
    Conn     net.Conn
    ID       string
    Username string
    Send     chan protocol.Message
}

func NewClient(conn net.Conn, id string, username string) *Client {
    return &Client{
        Conn:     conn,
        ID:       id,
        Username: username,
        Send:     make(chan protocol.Message, 256),
    }
}

// Close cleans up the client's resources
func (c *Client) Close() error {
    close(c.Send)
    return c.Conn.Close()
}

// IsConnected checks if the client is still connected
func (c *Client) IsConnected() bool {
    return c.Conn != nil
}