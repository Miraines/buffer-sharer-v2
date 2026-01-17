package network

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"buffer-sharer-app/internal/logging"
)

// AuthMessage - сообщение аутентификации для middleware
type AuthMessage struct {
	Type     string `json:"type"`
	Role     string `json:"role"`
	RoomCode string `json:"room_code,omitempty"`
}

// AuthResponse - ответ от middleware
type AuthResponse struct {
	Type     string `json:"type"`
	Success  bool   `json:"success"`
	RoomCode string `json:"room_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Client manages network connections to the middleware
type Client struct {
	logger      *logging.Logger
	host        string
	port        int
	role        string // "controller" or "client"
	roomCode    string // Room code (set by controller, input by client)

	mu          sync.RWMutex
	conn        net.Conn
	reader      *bufio.Reader
	connected   bool
	running     bool
	cancelFunc  context.CancelFunc

	// Reconnection settings
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration

	// Callbacks
	onMessage      func(*Message)
	onConnect      func()
	onDisconnect   func(error)
	onRoomCreated  func(string) // Called when controller creates a room
	onRoomJoined   func(string) // Called when client joins a room
	onAuthError    func(string) // Called on authentication error
}

// ClientConfig holds client configuration
type ClientConfig struct {
	Host                string
	Port                int
	Role                string // "controller" or "client"
	RoomCode            string // Room code for client role
	ReconnectDelayMs    int
	MaxReconnectDelayMs int
}

// NewClient creates a new network client
func NewClient(cfg ClientConfig, logger *logging.Logger) *Client {
	reconnectDelay := time.Duration(cfg.ReconnectDelayMs) * time.Millisecond
	if reconnectDelay <= 0 {
		reconnectDelay = 1 * time.Second
	}

	maxReconnectDelay := time.Duration(cfg.MaxReconnectDelayMs) * time.Millisecond
	if maxReconnectDelay <= 0 {
		maxReconnectDelay = 30 * time.Second
	}

	return &Client{
		logger:            logger,
		host:              cfg.Host,
		port:              cfg.Port,
		role:              cfg.Role,
		roomCode:          cfg.RoomCode,
		reconnectDelay:    reconnectDelay,
		maxReconnectDelay: maxReconnectDelay,
	}
}

// SetRoomCode sets the room code (for client role)
func (c *Client) SetRoomCode(code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.roomCode = code
}

// GetRoomCode returns the current room code
func (c *Client) GetRoomCode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.roomCode
}

// SetOnRoomCreated sets the callback for when a room is created (controller)
func (c *Client) SetOnRoomCreated(handler func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRoomCreated = handler
}

// SetOnRoomJoined sets the callback for when joining a room (client)
func (c *Client) SetOnRoomJoined(handler func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRoomJoined = handler
}

// SetOnAuthError sets the callback for authentication errors
func (c *Client) SetOnAuthError(handler func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAuthError = handler
}

// SetOnMessage sets the message handler callback
func (c *Client) SetOnMessage(handler func(*Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = handler
}

// SetOnConnect sets the connection callback
func (c *Client) SetOnConnect(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnect = handler
}

// SetOnDisconnect sets the disconnection callback
func (c *Client) SetOnDisconnect(handler func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = handler
}

// Connect establishes a connection to the middleware and authenticates
func (c *Client) Connect() error {
	c.logger.Debug("network", "Connect() called")
	c.mu.Lock()
	if c.connected {
		c.logger.Debug("network", "Connect(): already connected, returning nil")
		c.mu.Unlock()
		return nil
	}
	role := c.role
	roomCode := c.roomCode
	c.mu.Unlock()

	address := fmt.Sprintf("%s:%d", c.host, c.port)
	c.logger.Info("network", "Connecting to %s as %s (roomCode=%s)", address, role, roomCode)

	c.logger.Debug("network", "Dialing TCP to %s with 10s timeout...", address)
	startDial := time.Now()
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	dialElapsed := time.Since(startDial)
	if err != nil {
		c.logger.Error("network", "Failed to connect after %v: %v", dialElapsed, err)
		return err
	}
	c.logger.Debug("network", "TCP connection established in %v", dialElapsed)

	// Create buffered reader for auth response
	reader := bufio.NewReader(conn)

	// Send authentication message
	authMsg := AuthMessage{
		Type:     "auth",
		Role:     role,
		RoomCode: roomCode,
	}
	authData, _ := json.Marshal(authMsg)
	authData = append(authData, '\n')
	c.logger.Debug("network", "Sending auth message: %s", string(authData[:len(authData)-1]))

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(authData); err != nil {
		conn.Close()
		c.logger.Error("network", "Failed to send auth: %v", err)
		return err
	}
	c.logger.Debug("network", "Auth message sent, waiting for response...")

	// Read authentication response
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	respLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		c.logger.Error("network", "Failed to read auth response: %v", err)
		return err
	}
	c.logger.Debug("network", "Auth response received: %s", respLine)
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	var authResp AuthResponse
	if err := json.Unmarshal([]byte(respLine), &authResp); err != nil {
		conn.Close()
		c.logger.Error("network", "Invalid auth response: %v", err)
		return fmt.Errorf("invalid auth response")
	}

	if !authResp.Success {
		conn.Close()
		c.logger.Error("network", "Authentication failed: %s", authResp.Error)

		c.mu.RLock()
		onAuthError := c.onAuthError
		c.mu.RUnlock()
		if onAuthError != nil {
			onAuthError(authResp.Error)
		}

		return fmt.Errorf("auth failed: %s", authResp.Error)
	}

	// Save connection and room code
	c.mu.Lock()
	c.conn = conn
	c.reader = reader
	c.connected = true
	// For controller: always use the room code from server (it creates the room)
	// For client: keep the original room code (don't change if already set)
	if role == "controller" || c.roomCode == "" {
		c.roomCode = authResp.RoomCode
	}
	// Verify we got the expected room for client
	if role == "client" && c.roomCode != "" && authResp.RoomCode != c.roomCode {
		c.logger.Warn("network", "Server returned different room code: expected %s, got %s", c.roomCode, authResp.RoomCode)
	}
	onConnect := c.onConnect
	onRoomCreated := c.onRoomCreated
	onRoomJoined := c.onRoomJoined
	c.mu.Unlock()

	c.logger.Info("network", "Connected to room %s", authResp.RoomCode)

	// Notify callbacks
	if onConnect != nil {
		onConnect()
	}

	if role == "controller" && onRoomCreated != nil {
		onRoomCreated(authResp.RoomCode)
	} else if role == "client" && onRoomJoined != nil {
		onRoomJoined(authResp.RoomCode)
	}

	return nil
}

// Start starts the client with automatic reconnection
func (c *Client) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true

	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFunc = cancel
	c.mu.Unlock()

	go c.connectionLoop(ctx)
}

// connectionLoop handles connection and reconnection
func (c *Client) connectionLoop(ctx context.Context) {
	delay := c.reconnectDelay

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Try to connect
		if err := c.Connect(); err != nil {
			c.logger.Warn("network", "Connection failed, retrying in %v", delay)
			time.Sleep(delay)

			// Exponential backoff
			delay = delay * 2
			if delay > c.maxReconnectDelay {
				delay = c.maxReconnectDelay
			}
			continue
		}

		// Reset delay on successful connection
		delay = c.reconnectDelay

		// Start receiving messages
		c.receiveLoop(ctx)

		// If we get here, connection was lost
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// receiveLoop reads messages from the connection
func (c *Client) receiveLoop(ctx context.Context) {
	// Start keepalive goroutine
	keepaliveDone := make(chan struct{})
	go c.keepaliveLoop(ctx, keepaliveDone)
	defer close(keepaliveDone)

	consecutiveTimeouts := 0
	const maxConsecutiveTimeouts = 3 // Disconnect after 3 consecutive timeouts (~90 seconds)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		reader := c.reader
		c.mu.RUnlock()

		if conn == nil || reader == nil {
			return
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read message using buffered reader to avoid losing data buffered during auth
		msg, err := DecodeMessage(reader)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				consecutiveTimeouts++
				c.logger.Debug("network", "Read timeout (%d/%d)", consecutiveTimeouts, maxConsecutiveTimeouts)

				// Check if connection is still alive
				if consecutiveTimeouts >= maxConsecutiveTimeouts {
					c.logger.Warn("network", "Too many consecutive timeouts, assuming connection lost")
					c.mu.RLock()
					onDisconnect := c.onDisconnect
					c.mu.RUnlock()
					if onDisconnect != nil {
						onDisconnect(fmt.Errorf("connection timeout"))
					}
					return
				}
				continue
			}

			// Check if this is intentional shutdown (Stop() was called)
			c.mu.RLock()
			running := c.running
			onDisconnect := c.onDisconnect
			c.mu.RUnlock()

			// Only log error if we're still supposed to be running
			if running {
				c.logger.Error("network", "Read error: %v", err)
				if onDisconnect != nil {
					onDisconnect(err)
				}
			}
			// If !running, Stop() was called - this is expected, not an error

			return
		}

		// Reset timeout counter on successful read
		consecutiveTimeouts = 0

		// Handle ping messages (middleware keepalive)
		if msg.Type == TypePing {
			// Send pong response
			c.Send(&Message{Type: TypePong})
			continue
		}

		// Handle message
		c.mu.RLock()
		onMessage := c.onMessage
		c.mu.RUnlock()

		if onMessage != nil {
			onMessage(msg)
		}
	}
}

// keepaliveLoop sends periodic ping messages to keep connection alive
func (c *Client) keepaliveLoop(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second) // Send ping every 25 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.connected
			c.mu.RUnlock()

			if !connected {
				return
			}

			// Send ping
			if err := c.Send(&Message{Type: TypePing}); err != nil {
				c.logger.Debug("network", "Failed to send keepalive ping: %v", err)
			}
		}
	}
}

// Send sends a message to the middleware
func (c *Client) Send(msg *Message) error {
	c.logger.Debug("network", "Send() called with message type: %s", msg.Type)

	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		c.logger.Debug("network", "Send(): not connected (connected=%v, conn=%v)", connected, conn != nil)
		return ErrNotConnected
	}

	data, err := msg.Encode()
	if err != nil {
		c.logger.Error("network", "Send(): failed to encode message: %v", err)
		return err
	}
	c.logger.Debug("network", "Send(): message encoded, %d bytes", len(data))

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	startWrite := time.Now()
	n, err := conn.Write(data)
	writeElapsed := time.Since(startWrite)
	if err != nil {
		c.logger.Error("network", "Write error after %v: %v", writeElapsed, err)
		return err
	}

	c.logger.Debug("network", "Sent message type: %s (%d bytes in %v)", msg.Type, n, writeElapsed)
	return nil
}

// SendPayload creates and sends a message with the given type and payload
func (c *Client) SendPayload(msgType MessageType, payload interface{}) error {
	msg, err := NewMessage(msgType, payload)
	if err != nil {
		return err
	}
	return c.Send(msg)
}

// Disconnect closes the connection
func (c *Client) Disconnect() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.connected = false
	c.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
	if c.logger != nil {
		c.logger.Info("network", "Disconnected")
	}
}

// Stop stops the client
func (c *Client) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	cancelFunc := c.cancelFunc
	c.cancelFunc = nil
	conn := c.conn
	c.conn = nil
	c.connected = false
	c.mu.Unlock()

	// Cancel context first
	if cancelFunc != nil {
		cancelFunc()
	}

	// Close connection to unblock any reads
	if conn != nil {
		conn.Close()
	}

	if c.logger != nil {
		c.logger.Info("network", "Client stopped")
	}
}

// IsConnected returns true if connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// IsRunning returns true if the client is running
func (c *Client) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// GetAddress returns the middleware address
func (c *Client) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.host, c.port)
}

// SetAddress updates the middleware address
func (c *Client) SetAddress(host string, port int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.host = host
	c.port = port
}

// Errors
var (
	ErrNotConnected = &NetworkError{"not connected"}
)

// NetworkError represents a network error
type NetworkError struct {
	message string
}

func (e *NetworkError) Error() string {
	return e.message
}
