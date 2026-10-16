// minikv'nin giriş noktası: klavyeden komut okur, sonucu ekrana yazar.
// Asıl iş internal/command ve internal/store paketlerinde yapılıyor.
package main

import (
	"bufio"
	"fmt"
	"os"

	"minikv/internal/command"
	"minikv/internal/store"
)

func main() {
	s := store.New()
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("minikv | komutlar: SET <key> <value> · GET <key> · DEL <key> · EXIT")

	fmt.Print("> ")
	for sc.Scan() {
		reply, quit := command.Execute(s, sc.Text())
		if quit {
			return
		}
		if reply != "" {
			fmt.Println(reply)
		}
		fmt.Print("> ")
	}

	// Döngü bitti: ya giriş normal şekilde sona erdi, ya da okurken hata oldu.
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "okuma hatası:", err)
		os.Exit(1)
	}
}
