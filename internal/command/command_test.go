package command

import (
	"testing"

	"minikv/internal/store"
)

// Bir senaryoyu adım adım oynatıyoruz: her satır bir komut ve beklenen cevabı.
// Go'da buna "table-driven test" denir.
func TestExecuteScenario(t *testing.T) {
	s := store.New()

	steps := []struct {
		input string
		want  string
	}{
		{"SET isim alara", "OK"},
		{"GET isim", "alara"},
		{"set mesaj merhaba dünya", "OK"}, // küçük harf komut + boşluklu değer
		{"GET mesaj", "merhaba dünya"},
		{"DEL isim", "1"},
		{"DEL isim", "0"},
		{"GET isim", "(nil)"},
		{"   ", ""},
		{"SET eksik", "kullanım: SET <key> <value>"},
		{"GET", "kullanım: GET <key>"},
		{"FOO bar", "bilinmeyen komut: FOO"},
	}

	for _, step := range steps {
		got, quit := Execute(s, step.input)
		if quit {
			t.Fatalf("Execute(%q) programı kapatmamalıydı", step.input)
		}
		if got != step.want {
			t.Errorf("Execute(%q) = %q, want %q", step.input, got, step.want)
		}
	}
}

func TestExit(t *testing.T) {
	s := store.New()
	if _, quit := Execute(s, "exit"); !quit {
		t.Fatal("EXIT komutu quit=true döndürmeliydi")
	}
}
