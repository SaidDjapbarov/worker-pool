// tests/pool_test.go
// Этот тест проверяет работу нашего worker-pool:
// 1) корректную обработку задач,
// 2) динамическое Add/Remove воркеров,
// 3) graceful Close и отсутствие утечки горутин.

package tests

import (
	"bytes"
	"log"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/SaidDjapbarov/worker-pool/internal/workerpool"
)

// processedCount возвращает число обработанных задач,
// подсчитывая в логе вхождения " got:".
func processedCount(buf *bytes.Buffer) int {
	return strings.Count(buf.String(), " got:")
}

func TestPoolBehaviour(t *testing.T) {
	// Перенаправляем вывод логгера в буфер,
	// чтобы потом проверить, сколько раз воркеры реально вызвали process.
	var buf bytes.Buffer
	origOut := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(origOut)

	// Замеряем текущее число горутин, чтобы потом убедиться, что ничего не протекло.
	goroutinesBefore := runtime.NumGoroutine()

	// Создаём пул с 2 воркерами и буфером очереди на 64 задачи.
	p := workerpool.New(2, 64)

	const (
		initJobs    = 20  // первая порция задач
		stressJobs  = 500 // во время стресса
		finalJobs   = 30  // после стресса
		totalExpect = initJobs + stressJobs + finalJobs
	)

	// 1) Отправляем initJobs заданий
	for i := 0; i < initJobs; i++ {
		_ = p.Submit("init")
	}

	// 2) Параллельно добавляем/удаляем воркеров и шлём ещё задачи
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// Добавляем 5 воркеров, потом удаляем 4.
		defer wg.Done()
		p.Add(5)
		p.Remove(4)
	}()

	go func() {
		// Отправляем stressJobs задач подряд.
		defer wg.Done()
		for i := 0; i < stressJobs; i++ {
			_ = p.Submit("stress")
		}
	}()

	wg.Wait() // ждём, пока Add/Remove и Submit завершатся

	// 3) Ещё немного задач после стресса
	for i := 0; i < finalJobs; i++ {
		_ = p.Submit("final")
	}

	// 4) Закрываем пул и проверяем, что после Close новые задачи не принимаются
	p.Close()
	if err := p.Submit("late"); err == nil {
		t.Fatalf("expected error when submitting after Close")
	}

	// Считаем обработанные задачи и сравниваем с ожидаемым totalExpect
	got := processedCount(&buf)
	if got != totalExpect {
		t.Fatalf("processed %d jobs, want %d", got, totalExpect)
	}

	// 5) Убеждаемся, что число горутин не выросло (даём небольшой запас на GC)
	const leeway = 3
	if diff := runtime.NumGoroutine() - goroutinesBefore; diff > leeway {
		t.Fatalf("leaked %d goroutines", diff)
	}
}
