package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"minikv/internal/store"
)

func main() {
	s := store.New()
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("minikv | komutlar: SET <key> <value> · GET <key> · DEL <key> · EXIT")

	for {
		fmt.Print("> ")
		if !sc.Scan() {
			return
		}
		// "SET mesaj merhaba dünya" -> ["SET", "mesaj", "merhaba dünya"]
		parts := strings.SplitN(strings.TrimSpace(sc.Text()), " ", 3)

		switch strings.ToUpper(parts[0]) {
		case "":
			continue
		case "SET":
			if len(parts) != 3 {
				fmt.Println("kullanım: SET <key> <value>")
				continue
			}
			s.Set(parts[1], []byte(parts[2]))
			fmt.Println("OK")
		case "GET":
			if len(parts) != 2 {
				fmt.Println("kullanım: GET <key>")
				continue
			}
			v, err := s.Get(parts[1])
			if errors.Is(err, store.ErrNotFound) {
				fmt.Println("(nil)")
				continue
			}
			fmt.Println(string(v))
		case "DEL":
			if len(parts) != 2 {
				fmt.Println("kullanım: DEL <key>")
				continue
			}
			if s.Delete(parts[1]) {
				fmt.Println("1")
			} else {
				fmt.Println("0")
			}
		case "EXIT":
			return
		default:
			fmt.Println("bilinmeyen komut:", parts[0])
		}
	}
}
