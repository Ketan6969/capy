package storage

import "sync"

// Storage represents a mock web storage (localStorage / sessionStorage) implementation.
type Storage struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewStorage creates a new initialized Storage instance.
func NewStorage() *Storage {
	return &Storage{
		data: make(map[string]string),
	}
}

// GetItem retrieves the value associated with the given key.
func (s *Storage) GetItem(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	if !ok {
		return "" // return empty string (or goja will translate nil/empty properly)
	}
	return val
}

// SetItem stores the value associated with the given key.
func (s *Storage) SetItem(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// RemoveItem deletes the value associated with the given key.
func (s *Storage) RemoveItem(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Clear removes all stored key-value pairs.
func (s *Storage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]string)
}

// Key returns the name of the key at the specified index.
func (s *Storage) Key(index int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.data) {
		return ""
	}
	i := 0
	for k := range s.data {
		if i == index {
			return k
		}
		i++
	}
	return ""
}

// GetLength returns the number of items stored.
func (s *Storage) GetLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
