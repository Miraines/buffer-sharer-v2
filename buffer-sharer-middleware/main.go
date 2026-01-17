package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

// Middleware управляет подключениями и комнатами
type Middleware struct {
	rooms   map[string]*Room
	clients map[net.Conn]*ConnectedClient
	mu      sync.RWMutex
}

func NewMiddleware() *Middleware {
	return &Middleware{
		rooms:   make(map[string]*Room),
		clients: make(map[net.Conn]*ConnectedClient),
	}
}

// GenerateRoomCode генерирует случайный код комнаты
func GenerateRoomCode() string {
	bytes := make([]byte, 3) // 6 символов hex
	rand.Read(bytes)
	return strings.ToUpper(hex.EncodeToString(bytes))
}

// CreateRoom создаёт новую комнату для controller
func (m *Middleware) CreateRoom(controller *ConnectedClient) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Генерируем уникальный код
	var code string
	for {
		code = GenerateRoomCode()
		if _, exists := m.rooms[code]; !exists {
			break
		}
	}

	room := &Room{
		code:       code,
		controller: controller,
		createdAt:  time.Now(),
	}
	m.rooms[code] = room
	controller.roomCode = code

	log.Printf("[ROOM %s] Created by controller %s", code, controller.conn.RemoteAddr())
	return code
}

// JoinRoom присоединяет client к существующей комнате
func (m *Middleware) JoinRoom(client *ConnectedClient, code string) error {
	m.mu.Lock()

	code = strings.ToUpper(strings.TrimSpace(code))
	room, exists := m.rooms[code]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("room %s not found", code)
	}

	room.mu.Lock()

	if room.client != nil {
		room.mu.Unlock()
		m.mu.Unlock()
		return fmt.Errorf("room %s already has a client", code)
	}

	room.client = client
	client.roomCode = code

	log.Printf("[ROOM %s] Client %s joined", code, client.conn.RemoteAddr())

	// Copy controller reference before releasing locks to avoid blocking Write while holding room/middleware locks
	controller := room.controller

	room.mu.Unlock()
	m.mu.Unlock()

	// Уведомляем controller о подключении client (outside of room/middleware locks)
	if controller != nil {
		notifyMsg := map[string]interface{}{
			"type":    "notification",
			"message": "Client connected",
		}
		controller.mu.Lock()
		err := sendBinaryMessage(controller.conn, notifyMsg)
		controller.mu.Unlock()
		if err != nil {
			log.Printf("[ROOM %s] Failed to send notification: %v", code, err)
		}
	}

	return nil
}

// RejoinRoomAsController позволяет контроллеру переподключиться к существующей комнате
func (m *Middleware) RejoinRoomAsController(controller *ConnectedClient, code string) error {
	m.mu.Lock()

	code = strings.ToUpper(strings.TrimSpace(code))
	room, exists := m.rooms[code]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("room %s not found", code)
	}

	room.mu.Lock()

	// Если у комнаты уже есть контроллер - закрываем старое соединение и заменяем
	if room.controller != nil {
		log.Printf("[ROOM %s] Replacing old controller %s with new %s",
			code, room.controller.conn.RemoteAddr(), controller.conn.RemoteAddr())
		// Закрываем старое соединение синхронно (Close() быстрая операция)
		// Это вызовет RemoveClient в горутине HandleConnection
		oldController := room.controller
		room.controller = nil
		oldController.SafeClose()
	}

	// Присоединяем контроллер к комнате
	room.controller = controller
	controller.roomCode = code

	log.Printf("[ROOM %s] Controller %s rejoined", code, controller.conn.RemoteAddr())

	// Copy client reference before releasing locks to avoid blocking Write while holding room/middleware locks
	client := room.client

	room.mu.Unlock()
	m.mu.Unlock()

	// Уведомляем client о переподключении контроллера (outside of room/middleware locks)
	if client != nil {
		notifyMsg := map[string]interface{}{
			"type":    "notification",
			"message": "Controller reconnected",
		}
		client.mu.Lock()
		err := sendBinaryMessage(client.conn, notifyMsg)
		client.mu.Unlock()
		if err != nil {
			log.Printf("[ROOM %s] Failed to send notification: %v", code, err)
		}
	}

	return nil
}

// RemoveClient удаляет клиента и очищает комнату при необходимости
func (m *Middleware) RemoveClient(conn net.Conn) {
	m.mu.Lock()

	cc, ok := m.clients[conn]
	if !ok {
		m.mu.Unlock()
		return
	}

	// Variable to hold the target to notify after releasing locks
	var notifyTarget *ConnectedClient
	var notifyMessage string

	if cc.roomCode != "" {
		if room, exists := m.rooms[cc.roomCode]; exists {
			room.mu.Lock()
			if room.controller == cc {
				log.Printf("[ROOM %s] Controller disconnected, keeping room for reconnection", cc.roomCode)
				// НЕ удаляем комнату - оставляем для переподключения
				room.controller = nil
				// Copy client reference for notification
				if room.client != nil {
					notifyTarget = room.client
					notifyMessage = "Controller disconnected, waiting for reconnection"
				}
			} else if room.client == cc {
				log.Printf("[ROOM %s] Client disconnected", cc.roomCode)
				room.client = nil
				// Copy controller reference for notification
				if room.controller != nil {
					notifyTarget = room.controller
					notifyMessage = "Client disconnected"
				}
			}
			room.mu.Unlock()
		}
	}

	delete(m.clients, conn)
	log.Printf("[MIDDLEWARE] Connection closed: %s", conn.RemoteAddr())

	m.mu.Unlock()

	// Send notification outside of all locks to avoid deadlock
	if notifyTarget != nil {
		notifyMsg := map[string]interface{}{
			"type":    "notification",
			"message": notifyMessage,
		}
		notifyTarget.mu.Lock()
		err := sendBinaryMessage(notifyTarget.conn, notifyMsg)
		notifyTarget.mu.Unlock()
		if err != nil {
			log.Printf("[MIDDLEWARE] Failed to send notification: %v", err)
		}
	}
}

// GetTarget возвращает получателя сообщения
func (m *Middleware) GetTarget(sender *ConnectedClient) *ConnectedClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sender.roomCode == "" {
		return nil
	}

	room, exists := m.rooms[sender.roomCode]
	if !exists {
		return nil
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	if sender == room.client {
		return room.controller
	} else if sender == room.controller {
		return room.client
	}
	return nil
}

// HandleConnection обрабатывает новое подключение
func (m *Middleware) HandleConnection(conn net.Conn) {
	// cc will be set after auth succeeds; used by deferred close
	var cc *ConnectedClient
	defer func() {
		if cc != nil {
			cc.SafeClose()
		} else {
			conn.Close()
		}
	}()

	reader := bufio.NewReader(conn)

	// Ждём сообщение аутентификации (первая строка)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("[MIDDLEWARE] Auth read error from %s: %v", conn.RemoteAddr(), err)
		return
	}
	conn.SetReadDeadline(time.Time{}) // Убираем deadline

	// Парсим auth message
	var auth AuthMessage
	if err := json.Unmarshal([]byte(line), &auth); err != nil {
		log.Printf("[MIDDLEWARE] Invalid auth message from %s: %v", conn.RemoteAddr(), err)
		sendAuthResponse(conn, false, "", "Invalid auth message format")
		return
	}

	if auth.Type != "auth" {
		sendAuthResponse(conn, false, "", "Expected auth message")
		return
	}

	// Создаём клиента
	cc = &ConnectedClient{
		conn: conn,
		role: ClientRole(auth.Role),
	}

	m.mu.Lock()
	m.clients[conn] = cc
	m.mu.Unlock()

	defer m.RemoveClient(conn)

	// Обрабатываем аутентификацию в зависимости от роли
	switch cc.role {
	case RoleController:
		// Если контроллер указал код комнаты, пытаемся переподключиться
		if auth.RoomCode != "" {
			if err := m.RejoinRoomAsController(cc, auth.RoomCode); err == nil {
				sendAuthResponse(conn, true, cc.roomCode, "")
				log.Printf("[AUTH] Controller %s rejoined room %s", conn.RemoteAddr(), cc.roomCode)
				break
			}
			// Если не удалось переподключиться - создаём новую комнату
			log.Printf("[AUTH] Controller %s failed to rejoin room %s, creating new room", conn.RemoteAddr(), auth.RoomCode)
		}
		// Controller создаёт новую комнату
		roomCode := m.CreateRoom(cc)
		sendAuthResponse(conn, true, roomCode, "")
		log.Printf("[AUTH] Controller %s created room %s", conn.RemoteAddr(), roomCode)

	case RoleClient:
		// Client присоединяется к существующей комнате
		if auth.RoomCode == "" {
			sendAuthResponse(conn, false, "", "Room code required for client")
			return
		}
		if err := m.JoinRoom(cc, auth.RoomCode); err != nil {
			sendAuthResponse(conn, false, "", err.Error())
			return
		}
		sendAuthResponse(conn, true, cc.roomCode, "")
		log.Printf("[AUTH] Client %s joined room %s", conn.RemoteAddr(), cc.roomCode)

	default:
		sendAuthResponse(conn, false, "", "Invalid role, expected 'controller' or 'client'")
		return
	}

	// Основной цикл пересылки сообщений
	// ВАЖНО: Буфер должен быть достаточно большим для скриншотов (до 10MB)
	buf := make([]byte, 10*1024*1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		if n == 0 {
			continue
		}

		target := m.GetTarget(cc)
		if target == nil {
			log.Printf("[ROOM %s] No target for %s message", cc.roomCode, cc.role)
			continue
		}

		target.mu.Lock()
		_, err = target.conn.Write(buf[:n])
		target.mu.Unlock()

		if err != nil {
			log.Printf("[ROOM %s] Write error to %s: %v, closing target connection", cc.roomCode, target.role, err)
			// Close the target connection to trigger proper cleanup.
			// This will cause the target's HandleConnection to return on its next read,
			// which will call RemoveClient and update the room state.
			target.SafeClose()
			continue
		}

		log.Printf("[ROOM %s] Relayed %d bytes: %s -> %s",
			cc.roomCode, n, cc.role, target.role)
	}
}

func sendAuthResponse(conn net.Conn, success bool, roomCode, errMsg string) {
	resp := AuthResponse{
		Type:     "auth_response",
		Success:  success,
		RoomCode: roomCode,
		Error:    errMsg,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[AUTH] Failed to marshal auth response: %v", err)
		return
	}
	conn.Write(append(data, '\n'))
}

// sendBinaryMessage sends a message using binary protocol (4-byte length prefix + JSON)
// This is used for notifications to match client's expected protocol
func sendBinaryMessage(conn net.Conn, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Binary protocol: [4 bytes length][JSON message]
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)

	_, err = conn.Write(buf)
	return err
}

// cleanupStaleRooms удаляет комнаты без активности
func (m *Middleware) cleanupStaleRooms() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for code, room := range m.rooms {
			room.mu.RLock()
			// Удаляем комнаты старше 24 часов без client
			if room.client == nil && now.Sub(room.createdAt) > 24*time.Hour {
				log.Printf("[CLEANUP] Removing stale room %s", code)
				if room.controller != nil {
					room.controller.SafeClose()
				}
				delete(m.rooms, code)
			}
			room.mu.RUnlock()
		}
		m.mu.Unlock()
	}
}

func (m *Middleware) printStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		activeRooms := 0
		for _, room := range m.rooms {
			room.mu.RLock()
			if room.client != nil && room.controller != nil {
				activeRooms++
			}
			room.mu.RUnlock()
		}
		log.Printf("[STATS] Total rooms: %d, Active pairs: %d, Connections: %d",
			len(m.rooms), activeRooms, len(m.clients))
		m.mu.RUnlock()
	}
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	// Validate port number
	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Port must be between 1 and 65535", *port)
	}

	addr := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("╔════════════════════════════════════════╗")
	log.Printf("║     Buffer Sharer Middleware v4.0      ║")
	log.Printf("╠════════════════════════════════════════╣")
	log.Printf("║  Listening on %-24s ║", addr)
	log.Printf("╠════════════════════════════════════════╣")
	log.Printf("║  Room-based relay server               ║")
	log.Printf("║  Controller creates room -> gets code  ║")
	log.Printf("║  Client enters code -> joins room      ║")
	log.Printf("╚════════════════════════════════════════╝")
	log.Printf("")

	middleware := NewMiddleware()

	// Фоновые задачи
	go middleware.cleanupStaleRooms()
	go middleware.printStats()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down gracefully...")
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go middleware.HandleConnection(conn)
	}
}
