package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
)

// messageHeader используется для быстрого парсинга только поля type из JSON
type messageHeader struct {
	Type string `json:"type"`
}

// readBinaryMessage читает одно бинарное сообщение (4 байта длины + JSON тело).
// Возвращает полное сообщение (с префиксом длины) и тело отдельно.
func readBinaryMessage(conn net.Conn, buf []byte) (fullMsg []byte, body []byte, err error) {
	// Читаем 4 байта длины
	if _, err = io.ReadFull(conn, buf[:4]); err != nil {
		return nil, nil, err
	}
	length := binary.BigEndian.Uint32(buf[:4])

	if int(length) > len(buf)-4 {
		return nil, nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Читаем тело сообщения
	if _, err = io.ReadFull(conn, buf[4:4+length]); err != nil {
		return nil, nil, err
	}

	total := 4 + int(length)
	return buf[:total], buf[4:total], nil
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
