package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

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
