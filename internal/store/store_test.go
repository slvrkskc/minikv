package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestSetGet(t *testing.T) {
	s := New()
	s.Set("name", []byte("alara"))

	got, err := s.Get("name")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if string(got) != "alara" {
		t.Fatalf("got %q, want %q", got, "alara")
	}
}

func TestGetMissing(t *testing.T) {
	s := New()
	if _, err := s.Get("yok"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestOverwrite(t *testing.T) {
	s := New()
	s.Set("k", []byte("v1"))
	s.Set("k", []byte("v2"))

	got, _ := s.Get("k")
	if string(got) != "v2" {
		t.Fatalf("got %q, want %q", got, "v2")
	}
}

func TestDelete(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))

	if !s.Delete("k") {
		t.Fatal("var olan key silinince true dönmeliydi")
	}
	if s.Delete("k") {
		t.Fatal("olmayan key silinince false dönmeliydi")
	}
	if _, err := s.Get("k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("silinen key hâlâ okunabiliyor: %v", err)
	}
}

func TestValueIsCopied(t *testing.T) {
	s := New()
	buf := []byte("hello")
	s.Set("k", buf)
	buf[0] = 'J' // dışarıdaki slice'ı değiştiriyoruz

	got, _ := s.Get("k")
	if string(got) != "hello" {
		t.Fatalf("store dış değişiklikten etkilendi: %q", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			s.Set(key, []byte("v"))
			if _, err := s.Get(key); err != nil {
				t.Errorf("Get(%s): %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	if s.Len() != 100 {
		t.Fatalf("got %d keys, want 100", s.Len())
	}
}
