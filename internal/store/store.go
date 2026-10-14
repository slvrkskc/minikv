// Package store, minikv'nin bellek içi (in-memory) veri katmanıdır.
package store

import (
	"errors"
	"sync"
)

// ErrNotFound, istenen anahtar yoksa döner.
var ErrNotFound = errors.New("key not found")

// Store, eşzamanlı (concurrent) kullanıma güvenli bir key-value deposudur.
type Store struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// New, boş bir Store oluşturur.
func New() *Store {
	return &Store{data: make(map[string][]byte)}
}

// Set, key'e value atar. Varsa üzerine yazar.
func (s *Store) Set(key string, value []byte) {
	// Çağıranın slice'ı sonradan değiştirmesi depoyu etkilemesin.
	v := make([]byte, len(value))
	copy(v, value)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = v
}

// Get, key'in değerini döner. Yoksa ErrNotFound.
func (s *Store) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	// İç veriyi dışarıya açmıyoruz.
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

// Delete, key'i siler. Key gerçekten vardıysa true döner.
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.data[key]
	delete(s.data, key)
	return ok
}

// Len, depodaki anahtar sayısını döner.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
