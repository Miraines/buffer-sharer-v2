package main

import (
	"net"
	"sync"
	"time"
)

// ClientRole определяет роль подключённого клиента
type ClientRole string

const (
	RoleController ClientRole = "controller"
	RoleClient     ClientRole = "client"
)

// AuthMessage - сообщение аутентификации
type AuthMessage struct {
	Type     string `json:"type"`      // "auth"
	Role     string `json:"role"`      // "controller" или "client"
	RoomCode string `json:"room_code"` // Код комнаты
}

// AuthResponse - ответ на аутентификацию
type AuthResponse struct {
	Type     string `json:"type"`     // "auth_response"
	Success  bool   `json:"success"`
	RoomCode string `json:"room_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ConnectedClient представляет подключённого клиента
type ConnectedClient struct {
	conn      net.Conn
	role      ClientRole
	roomCode  string
	mu        sync.Mutex
	closeOnce sync.Once
}

// SafeClose closes the connection exactly once, preventing double close panics.
// It is safe to call this method multiple times from different goroutines.
func (cc *ConnectedClient) SafeClose() {
	cc.closeOnce.Do(func() {
		cc.conn.Close()
	})
}

// Room представляет комнату с парой client-controller
type Room struct {
	code       string
	controller *ConnectedClient
	client     *ConnectedClient
	createdAt  time.Time
	mu         sync.RWMutex
}
