package storage

import (
	"errors"
	"sort"
	"sync"
	"time"

	"notes/internal/note"
)

var ErrNotFound = errors.New("note not found")

type Storage struct {
	mu    sync.RWMutex
	notes map[note.Title]note.Note
}

func New() *Storage {
	return &Storage{notes: make(map[note.Title]note.Note)}
}

func (s *Storage) Create(key string, title note.Title, value string) (note.Note, error) {
	if err := note.ValidateTitle(title); err != nil {
		return note.Note{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := note.Note{
		Key:       key,
		Title:     title,
		Value:     value,
		UpdatedAt: time.Now(),
	}
	s.notes[title] = n
	return n, nil
}

func (s *Storage) Get(title note.Title) (note.Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notes[title]
	if !ok {
		return note.Note{}, ErrNotFound
	}
	return n, nil
}

func (s *Storage) Update(title note.Title, value string) (note.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[title]
	if !ok {
		return note.Note{}, ErrNotFound
	}
	n.Value = value
	n.UpdatedAt = time.Now()
	s.notes[title] = n
	return n, nil
}

func (s *Storage) Delete(title note.Title) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notes[title]; !ok {
		return ErrNotFound
	}
	delete(s.notes, title)
	return nil
}

func (s *Storage) List() []note.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]note.Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}
