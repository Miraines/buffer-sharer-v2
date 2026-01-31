package network

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// MaxMessageSize is the maximum allowed message size (100MB)
// This prevents DoS attacks where a malicious client sends a huge length prefix
const MaxMessageSize = 100 * 1024 * 1024 // 100MB

// ErrMessageTooLarge is returned when a message exceeds MaxMessageSize
var ErrMessageTooLarge = errors.New("message size exceeds maximum allowed")

// MessageType represents the type of message being sent
type MessageType string

const (
	// Message types
	TypeScreenshot  MessageType = "screenshot"
	TypeText        MessageType = "text"
	TypeFile        MessageType = "file"
	TypeClipboard   MessageType = "clipboard"
	TypeCommand     MessageType = "command"
	TypeStatus      MessageType = "status"
	TypeInputMode   MessageType = "input_mode"
	TypeKeyboardBuf MessageType = "keyboard_buffer"
	TypePing        MessageType = "ping"
	TypePong        MessageType = "pong"

	// Overlay features
	TypeNotification MessageType = "notification"

	TypeCursorMove  MessageType = "cursor_move"
	TypeCursorClick MessageType = "cursor_click"
	TypeCursorShow  MessageType = "cursor_show"
	TypeCursorHide  MessageType = "cursor_hide"

	TypeHintShow  MessageType = "hint_show"
	TypeHintHide  MessageType = "hint_hide"
	TypeHintClear MessageType = "hint_clear"

	TypeDrawStart MessageType = "draw_start"
	TypeDrawMove  MessageType = "draw_move"
	TypeDrawEnd   MessageType = "draw_end"
	TypeDrawClear MessageType = "draw_clear"
	TypeDrawUndo  MessageType = "draw_undo"

	TypeTextOverlay      MessageType = "text_overlay"
	TypeTextOverlayClear MessageType = "text_overlay_clear"
	TypeHintCollapse     MessageType = "hint_collapse"
	TypeHintExpand       MessageType = "hint_expand"
	TypeHintDelete       MessageType = "hint_delete"
	TypeTextOverlayDel   MessageType = "text_overlay_delete"
)

// Message represents a protocol message
type Message struct {
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// ScreenshotPayload contains screenshot data
type ScreenshotPayload struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
	Data     []byte `json:"data"`
	Quality  int    `json:"quality,omitempty"`
}

// TextPayload contains text data for injection
type TextPayload struct {
	Text      string `json:"text"`
	Immediate bool   `json:"immediate,omitempty"`
}

// FilePayload contains file transfer data
type FilePayload struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type,omitempty"`
}

// ClipboardPayload contains clipboard data
type ClipboardPayload struct {
	Text      string `json:"text,omitempty"`
	ImageData []byte `json:"image_data,omitempty"`
	Format    string `json:"format"`
}

// CommandPayload contains a command to execute
type CommandPayload struct {
	Command string            `json:"command"`
	Args    map[string]string `json:"args,omitempty"`
}

// StatusPayload contains status information
type StatusPayload struct {
	Connected    bool   `json:"connected"`
	Mode         string `json:"mode"`
	InputMode    bool   `json:"input_mode"`
	Message      string `json:"message,omitempty"`
}

// InputModePayload contains input mode state
type InputModePayload struct {
	Enabled bool `json:"enabled"`
}

// NotificationPayload contains toast notification data
type NotificationPayload struct {
	Text     string `json:"text"`
	Type     string `json:"type"`     // "info", "success", "warning", "error"
	Duration int    `json:"duration"` // ms, 0 = default (3000)
}

// CursorPayload contains cursor position data (relative 0.0-1.0)
type CursorPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// HintPayload contains tooltip/hint data
type HintPayload struct {
	ID       string  `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Text     string  `json:"text"`
	Duration int     `json:"duration"` // seconds, 0 = permanent
}

// DrawStartPayload contains draw start data
type DrawStartPayload struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Color     string  `json:"color"`     // hex #FF0000
	Thickness float64 `json:"thickness"` // relative
	Tool      string  `json:"tool"`      // "brush", "eraser", "arrow", "rect", "circle", "oval", "line", "checkmark", "cross"
}

// DrawMovePayload contains draw move data
type DrawMovePayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// DrawEndPayload contains draw end data
type DrawEndPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// KeyboardBufferPayload contains buffered keyboard input
type KeyboardBufferPayload struct {
	Buffer string `json:"buffer"`
	Clear  bool   `json:"clear,omitempty"`
}

// TextOverlayPayload contains text overlay data
type TextOverlayPayload struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Text  string  `json:"text"`
	Color string  `json:"color"`
	Size  float64 `json:"size"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	var payloadBytes json.RawMessage
	var err error

	if payload != nil {
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	return &Message{
		Type:      msgType,
		Payload:   payloadBytes,
		Timestamp: currentTimestamp(),
	}, nil
}

// ParsePayload unmarshals the payload into the given struct
func (m *Message) ParsePayload(v interface{}) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}

// Encode encodes a message for transmission
// Format: [4 bytes length][JSON message]
func (m *Message) Encode() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	// Create buffer with length prefix
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)

	return buf, nil
}

// DecodeMessage reads and decodes a message from a reader
func DecodeMessage(r io.Reader) (*Message, error) {
	// Read length prefix
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	// Validate message size to prevent DoS attacks
	if length > MaxMessageSize {
		return nil, fmt.Errorf("%w: %d bytes (max: %d)", ErrMessageTooLarge, length, MaxMessageSize)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	// Unmarshal message
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// currentTimestamp returns the current Unix timestamp in milliseconds
func currentTimestamp() int64 {
	return time.Now().UnixMilli()
}
