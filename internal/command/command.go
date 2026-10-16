// Package command, kullanıcının yazdığı metin komutlarını (SET, GET, DEL, EXIT)
// çözer ve store üzerinde çalıştırır.
//
// Bu mantığı main.go'dan ayırmamızın sebebi: hem komut satırı hem de
// ileride yazacağımız TCP sunucusu aynı paketi kullanacak. Ayrıca bu sayede
// komutları klavyeye ihtiyaç duymadan test edebiliyoruz.
package command

import (
	"errors"
	"strings"

	"minikv/internal/store"
)

// Execute, tek satırlık bir komutu çalıştırır.
// İki şey döndürür: kullanıcıya gösterilecek cevap ve programın kapanıp kapanmayacağı.
func Execute(s *store.Store, line string) (reply string, quit bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	// "SET mesaj merhaba dünya" -> ["SET", "mesaj", "merhaba dünya"]
	parts := strings.SplitN(line, " ", 3)
	cmd := strings.ToUpper(parts[0])

	switch cmd {
	case "SET":
		if len(parts) != 3 {
			return "kullanım: SET <key> <value>", false
		}
		s.Set(parts[1], []byte(parts[2]))
		return "OK", false

	case "GET":
		if len(parts) != 2 {
			return "kullanım: GET <key>", false
		}
		v, err := s.Get(parts[1])
		if errors.Is(err, store.ErrNotFound) {
			return "(nil)", false
		}
		return string(v), false

	case "DEL":
		if len(parts) != 2 {
			return "kullanım: DEL <key>", false
		}
		if s.Delete(parts[1]) {
			return "1", false
		}
		return "0", false

	case "EXIT":
		return "", true

	default:
		return "bilinmeyen komut: " + parts[0], false
	}
}
