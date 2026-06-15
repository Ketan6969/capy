package storage

import "testing"

func TestStorage(t *testing.T) {
	s := NewStorage()

	if s.GetLength() != 0 {
		t.Errorf("Expected length 0, got %d", s.GetLength())
	}

	s.SetItem("foo", "bar")
	s.SetItem("hello", "world")

	if s.GetLength() != 2 {
		t.Errorf("Expected length 2, got %d", s.GetLength())
	}

	if s.GetItem("foo") != "bar" {
		t.Errorf("Expected 'bar', got '%s'", s.GetItem("foo"))
	}

	s.RemoveItem("foo")
	if s.GetLength() != 1 {
		t.Errorf("Expected length 1, got %d", s.GetLength())
	}
	if s.GetItem("foo") != "" {
		t.Errorf("Expected empty string for removed item, got '%s'", s.GetItem("foo"))
	}

	keyName := s.Key(0)
	if keyName != "hello" {
		t.Errorf("Expected key at index 0 to be 'hello', got '%s'", keyName)
	}

	s.Clear()
	if s.GetLength() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", s.GetLength())
	}
}
