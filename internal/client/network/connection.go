// internal/client/network/connection.go
package network

import (
    "net"
    "time"
)

type Connection struct {
    conn net.Conn
    Send chan []byte
}

func NewConnection(address string) (*Connection, error) {
    dialer := net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }

    conn, err := dialer.Dial("tcp", address)
    if err != nil {
        return nil, err
    }

    // TCP configurations
    if tcpConn, ok := conn.(*net.TCPConn); ok {
        tcpConn.SetKeepAlive(true)
        tcpConn.SetKeepAlivePeriod(30 * time.Second)
        tcpConn.SetNoDelay(true)
    }

    return &Connection{
        conn: conn,
        Send: make(chan []byte, 100),
    }, nil
}

func (c *Connection) Write(data []byte) (n int, err error) {
    return c.conn.Write(data)
}

func (c *Connection) Read(data []byte) (n int, err error) {
    return c.conn.Read(data)
}

func (c *Connection) Close() error {
    close(c.Send)
    return c.conn.Close()
}

func (c *Connection) GetUnderlyingConn() net.Conn {
    return c.conn
}

func (c *Connection) RemoteAddr() net.Addr {
    return c.conn.RemoteAddr()
}

func (c *Connection) LocalAddr() net.Addr {
    return c.conn.LocalAddr()
}

func (c *Connection) SetDeadline(t time.Time) error {
    return c.conn.SetDeadline(t)
}

func (c *Connection) SetReadDeadline(t time.Time) error {
    return c.conn.SetReadDeadline(t)
}

func (c *Connection) SetWriteDeadline(t time.Time) error {
    return c.conn.SetWriteDeadline(t)
}
