package store

import (
	"sync"
	"time"

	"github.com/nubank/lola-ia-backend/internal"
)

type MemoryStore struct {
	mu        sync.Mutex
	messages  []internal.Message
	knowledge []internal.KnowledgeFile
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{messages: make([]internal.Message, 0, 64)}
}

func (s *MemoryStore) All() []internal.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]internal.Message, len(s.messages))
	copy(cp, s.messages)
	return cp
}

func (s *MemoryStore) Append(msg internal.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

func (s *MemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
}

func SeedAssistantHello(s *MemoryStore, text string) {
	s.Append(internal.Message{
		Role:      internal.RoleAssistant,
		Content:   text,
		CreatedAt: time.Now(),
	})
}

func (s *MemoryStore) AddFiles(files []internal.KnowledgeFile) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	// simple de-dup por nombre: el nuevo reemplaza
	nameToIdx := make(map[string]int)
	for i, f := range s.knowledge {
		nameToIdx[f.Name] = i
	}
	for _, f := range files {
		if idx, ok := nameToIdx[f.Name]; ok {
			s.knowledge[idx] = f
		} else {
			s.knowledge = append(s.knowledge, f)
			nameToIdx[f.Name] = len(s.knowledge) - 1
		}
	}
	return len(s.knowledge)
}

func (s *MemoryStore) ListFiles() []internal.KnowledgeFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]internal.KnowledgeFile, len(s.knowledge))
	copy(cp, s.knowledge)
	return cp
}

func (s *MemoryStore) RemoveFile(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.knowledge[:0]
	for _, f := range s.knowledge {
		if f.Name != name {
			out = append(out, f)
		}
	}
	s.knowledge = out
	return len(s.knowledge)
}

func (s *MemoryStore) ClearFiles() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.knowledge = s.knowledge[:0]
}
