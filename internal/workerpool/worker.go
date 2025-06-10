// Этот файл определяет тип worker — горутину, которая считывает задачи из общей очереди,
// обрабатывает их и корректно завершает работу при получении сигнала остановки.
package workerpool

import (
	"context"
	"log"
	"sync"
)

// worker хранит своё ID, канал задач, контекст отмены и указатель на WaitGroup,
// чтобы пул знал, когда эта горутина завершилась.
type worker struct {
	id     int
	jobs   <-chan string
	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// newWorker создаёт новый worker с уникальным ID и собственным контекстом-отменой,
// прибавляет его к WaitGroup и запускает основной цикл в отдельной горутине.
func newWorker(id int, jobs <-chan string, wg *sync.WaitGroup, parent context.Context) *worker {
	ctx, cancel := context.WithCancel(parent)
	w := &worker{
		id:     id,
		jobs:   jobs,
		wg:     wg,
		ctx:    ctx,
		cancel: cancel,
	}
	wg.Add(1)
	go w.loop()
	return w
}

// stop даёт команду worker-у «уйти», отменяя его контекст.
func (w *worker) stop() {
	w.cancel()
}

// loop работает до тех пор, пока есть задачи или не придёт сигнал отмены.
// Сначала слушает канал jobs, а при отмене сначала опустошает очередь, чтобы не потерять задания.
func (w *worker) loop() {
	defer w.wg.Done()

	for {
		select {
		// получаем новую задачу
		case job, ok := <-w.jobs:
			if !ok {
				// очередь закрыта, выходим навсегда
				log.Printf("[worker %d] shutting down\n", w.id)
				return
			}
			w.process(job)

		// пришёл сигнал отмены (отдельный stop или общий Close)
		case <-w.ctx.Done():
			// после отмены жадно забираем всё, что осталось в канале
			for {
				select {
				case job, ok := <-w.jobs:
					if !ok {
						// канал пуст и закрыт — выходим
						log.Printf("[worker %d] shutting down\n", w.id)
						return
					}
					w.process(job)
				default:
					// очередь пуста — завершаем работу
					log.Printf("[worker %d] shutting down\n", w.id)
					return
				}
			}
		}
	}
}

// process демонстрирует работу над задачей — здесь просто логируем полученные данные.
func (w *worker) process(data string) {
	log.Printf("[worker %d] got: %s\n", w.id, data)
}
