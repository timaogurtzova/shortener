package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Observer получает события аудита от издателя.
type Observer interface {
	ID() string
	Update(context.Context, Event) error
}

// Publisher хранит подписки и уведомляет наблюдателей о событиях аудита.
type Publisher struct {
	mu        sync.RWMutex
	observers map[string]Observer
}

// NewPublisher создаёт пустой издатель событий аудита.
func NewPublisher() *Publisher {
	return &Publisher{
		observers: make(map[string]Observer),
	}
}

// Register подписывает наблюдателя на будущие события аудита.
func (p *Publisher) Register(observer Observer) {
	if p == nil || observer == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.observers[observer.ID()] = observer
}

// Deregister удаляет подписку наблюдателя по ID.
func (p *Publisher) Deregister(id string) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

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

	observers := p.snapshot()
	errs := make([]error, 0, len(observers))

	for _, observer := range observers {
		if err := observer.Update(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", observer.ID(), err))
		}
	}

	return errors.Join(errs...)
}

func (p *Publisher) snapshot() []Observer {
	p.mu.RLock()
	defer p.mu.RUnlock()

	observers := make([]Observer, 0, len(p.observers))
	for _, observer := range p.observers {
		observers = append(observers, observer)
	}

	return observers
}
