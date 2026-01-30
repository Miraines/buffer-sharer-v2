package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"time"
)

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
		fullMsg, body, err := readBinaryMessage(conn, buf)
		if err != nil {
			return
		}

		// Парсим тип сообщения
		var header messageHeader
		json.Unmarshal(body, &header)

		// Ping обрабатываем самостоятельно — отвечаем pong отправителю
		if header.Type == "ping" {
			cc.mu.Lock()
			sendBinaryMessage(cc.conn, map[string]interface{}{
				"type":      "pong",
				"timestamp": time.Now().UnixMilli(),
			})
			cc.mu.Unlock()
			continue
		}

		target := m.GetTarget(cc)
		if target == nil {
			log.Printf("[ROOM %s] No target for %s message (type=%s)", cc.roomCode, cc.role, header.Type)
			continue
		}

		target.mu.Lock()
		_, err = target.conn.Write(fullMsg)
		target.mu.Unlock()

		if err != nil {
			log.Printf("[ROOM %s] Write error to %s: %v, closing target connection", cc.roomCode, target.role, err)
			// Close the target connection to trigger proper cleanup.
			// This will cause the target's HandleConnection to return on its next read,
			// which will call RemoveClient and update the room state.
			target.SafeClose()
			continue
		}

		log.Printf("[ROOM %s] Relayed %d bytes: %s -> %s (type=%s)",
			cc.roomCode, len(fullMsg), cc.role, target.role, header.Type)
	}
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
