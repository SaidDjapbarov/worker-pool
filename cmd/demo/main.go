// Этот файл запускает интерактивную CLI-демонстрацию работы worker-pool.
// Пользователь вводит строки, пул передаёт их воркерам, те параллельно их обрабатывают и выводят в лог.
// По Ctrl+C приложение корректно завершает работу всех воркеров и выходит с кодом 0.

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/SaidDjapbarov/worker-pool/internal/workerpool"
)

// promptWriter позволяет после каждого лог-сообщения снова вывести приглашение "> ".
type promptWriter struct{ w io.Writer }

// Write печатает p и, если сообщение заканчивается "\n", рисует новый "> " в stdout.
func (pw promptWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if strings.HasSuffix(string(p), "\n") {
		fmt.Fprint(os.Stdout, "> ")
	}
	return n, err
}

func main() {
	// Перенаправляем стандартный логгер в promptWriter
	log.SetFlags(log.LstdFlags)
	log.SetOutput(promptWriter{os.Stderr})

	// Создаём пул с 3 воркерами и буфером 10 заданий
	pool := workerpool.New(3, 10)

	// Поприветствуем пользователя и покажем, как выйти
	fmt.Println("Simple worker-pool demo. Press Ctrl+C to exit.")
	fmt.Print("> ")

	// Настраиваем перехват сигналов Ctrl+C (SIGINT/SIGTERM)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-sig:
			// Пользователь нажал Ctrl+C — останавливаем пул и выходим
			fmt.Println("\nStopping…")
			fmt.Print("> ")
			pool.Close()
			return

		default:
			// Читаем строку из stdin
			if !scanner.Scan() {
				// EOF (например, Ctrl+D) — тоже закрываем пул и выходим
				pool.Close()
				return
			}
			text := strings.TrimSpace(scanner.Text())
			if text != "" {
				// Отправляем непустую строку в пул
				if err := pool.Submit(text); err != nil {
					log.Printf("Error submitting job: %v\n", err)
				}
			} else {
				// Если строка оказалась пустой, само приглашение не перерисует логгер
				fmt.Print("> ")
			}
		}
	}
}
