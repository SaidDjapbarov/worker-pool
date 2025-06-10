// Package workerpool предоставляет простой пул воркеров с возможностью
// добавлять и убирать воркеров на лету и корректно завершать их работу.
package workerpool

import (
	"context"
	"errors"
	"sync"
)

// ErrClosed возвращается при попытке отправить задачу в уже закрытый пул.
var ErrClosed = errors.New("workerpool: pool closed")

// Pool управляет очередью заданий и набором воркеров.
type Pool struct {
	mu      sync.Mutex     // блокирует изменение workers, closed и nextID
	jobs    chan string    // очередь задач для воркеров
	wg      sync.WaitGroup // ждёт завершения всех воркеров при Close
	workers []*worker      // активные воркеры

	ctx    context.Context    // общий контекст для остановки воркеров
	cancel context.CancelFunc // функция отмены ctx
	nextID int                // уникальный идентификатор для нового воркера
	closed bool               // флаг, что пул закрыт и новые задачи не принимаются
}

// New создаёт пул с заданным числом воркеров и размером буфера очереди.
// Если initial <= 0, происходит panic, потому что пул без воркеров бесполезен.
func New(initial, buf int) *Pool {
	if initial <= 0 {
		panic("workerpool: initial must be > 0")
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		jobs:   make(chan string, buf),
		ctx:    ctx,
		cancel: cancel,
	}

	// запускаем initial воркеров
	for i := 0; i < initial; i++ {
		w := newWorker(p.nextID, p.jobs, &p.wg, p.ctx)
		p.workers = append(p.workers, w)
		p.nextID++
	}

	return p
}

// Submit добавляет новую задачу в очередь.
// Если пул уже закрыт, вернётся ErrClosed.
func (p *Pool) Submit(data string) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()

	if closed {
		return ErrClosed
	}

	select {
	case p.jobs <- data:
		return nil
	case <-p.ctx.Done():
		// пул закрыли в этот момент
		return ErrClosed
	}
}

// Add поднимает ещё n воркеров.
// Нулевые или отрицательные значения игнорируются.
func (p *Pool) Add(n int) {
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		w := newWorker(p.nextID, p.jobs, &p.wg, p.ctx)

		p.mu.Lock()
		p.workers = append(p.workers, w)
		p.nextID++
		p.mu.Unlock()
	}
}

// Remove аккуратно останавливает n последних воркеров.
// Если n больше количества активных воркеров, остановятся все.
func (p *Pool) Remove(n int) {
	if n <= 0 {
		return
	}

	p.mu.Lock()
	if n > len(p.workers) {
		n = len(p.workers)
	}
	// выбираем последних n воркеров
	victims := p.workers[len(p.workers)-n:]
	p.workers = p.workers[:len(p.workers)-n]
	p.mu.Unlock()

	// даём команду на остановку каждому из них
	for _, w := range victims {
		w.stop()
	}
}

// Close закрывает очередь, ждёт, пока все воркеры обработают оставшиеся задачи,
// а затем отменяет контекст, запрещая новые Submit.
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.jobs) // сначала закрываем канал, чтобы воркеры увидели конец очереди
	p.mu.Unlock()

	p.wg.Wait() // ждём, пока все задачи будут обработаны
	p.cancel()  // прерываем любые висящие select в воркерах

	// чистим слайс, чтобы освободить память и запретить повторное использование
	p.mu.Lock()
	p.workers = nil
	p.mu.Unlock()
}
