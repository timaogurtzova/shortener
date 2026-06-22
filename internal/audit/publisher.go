package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrPublisherClosed возвращается при попытке отправить событие через закрытый Publisher.
var ErrPublisherClosed = errors.New("audit publisher is closed")

// Observer получает события аудита от издателя.
type Observer interface {
	// ID возвращает стабильный идентификатор наблюдателя.
	ID() string

	// Update обрабатывает одно событие аудита.
	Update(context.Context, Event) error
}

type closeObserver interface {
	Close() error
}

// Publisher хранит подписки и уведомляет наблюдателей о событиях аудита.
type Publisher struct {
	mu             sync.Mutex
	closeCond      *sync.Cond
	observers      map[string]Observer
	closed         bool
	activeNotifies int
}

// NewPublisher создаёт пустой издатель событий аудита.
func NewPublisher() *Publisher {
	publisher := &Publisher{
		observers: make(map[string]Observer),
	}
	publisher.closeCond = sync.NewCond(&publisher.mu)

	return publisher
}

// Register подписывает наблюдателя на будущие события аудита.
func (p *Publisher) Register(observer Observer) {
	if p == nil || observer == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.observers[observer.ID()] = observer
}

// Deregister удаляет подписку наблюдателя по ID.
func (p *Publisher) Deregister(id string) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	delete(p.observers, id)
}

// Notify отправляет событие всем зарегистрированным наблюдателям.
func (p *Publisher) Notify(ctx context.Context, event Event) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	observers, ok := p.startNotify()
	if !ok {
		return ErrPublisherClosed
	}
	defer p.finishNotify()

	errCh := make(chan error, len(observers))
	var wg sync.WaitGroup

	for _, observer := range observers {
		observer := observer

		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := observer.Update(ctx, event); err != nil {
				errCh <- fmt.Errorf("%s: %w", observer.ID(), err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	errs := make([]error, 0, len(observers))
	for err := range errCh {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// Close закрывает зарегистрированных наблюдателей, которые владеют внешними ресурсами.
func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}

	observers := p.close()
	errs := make([]error, 0, len(observers))

	for _, observer := range observers {
		closer, ok := observer.(closeObserver)
		if !ok {
			continue
		}

		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", observer.ID(), err))
		}
	}

	return errors.Join(errs...)
}

func (p *Publisher) startNotify() ([]Observer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, false
	}

	observers := make([]Observer, 0, len(p.observers))
	for _, observer := range p.observers {
		observers = append(observers, observer)
	}

	p.activeNotifies++
	return observers, true
}

func (p *Publisher) finishNotify() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.activeNotifies--
	if p.closed && p.activeNotifies == 0 {
		p.closeCond.Broadcast()
	}
}

func (p *Publisher) close() []Observer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	for p.activeNotifies > 0 {
		p.closeCond.Wait()
	}

	observers := make([]Observer, 0, len(p.observers))
	for _, observer := range p.observers {
		observers = append(observers, observer)
	}

	return observers
}
