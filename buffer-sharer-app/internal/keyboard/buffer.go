package keyboard

import (
	"sync"
	"unicode/utf8"
)

// Buffer stores text that will be typed when triggered
type Buffer struct {
	mu      sync.RWMutex
	content string
}

// NewBuffer creates a new text buffer
func NewBuffer() *Buffer {
	return &Buffer{}
}

// Set sets the buffer content
func (b *Buffer) Set(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.content = text
}

// Append appends text to the buffer
func (b *Buffer) Append(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.content += text
}

// Get returns the current buffer content
func (b *Buffer) Get() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.content
}

// Clear clears the buffer and returns the previous content
func (b *Buffer) Clear() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	content := b.content
	b.content = ""
	return content
}

// IsEmpty returns true if the buffer is empty
func (b *Buffer) IsEmpty() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.content == ""
}

// Length returns the length of the buffer content in runes (characters)
func (b *Buffer) Length() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return utf8.RuneCountInString(b.content)
}
