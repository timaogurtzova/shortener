package audit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var ErrDispatcherClosed = errors.New("audit dispatcher is closed")

// Dispatcher принимает события аудита и доставляет их подписчикам из фонового воркера.
type Dispatcher struct {
	publisher       *Publisher
	events          chan Event
	done            chan struct{}
	mu              sync.Mutex
	closed          bool
	closeOnce       sync.Once
	wg              sync.WaitGroup
	deliveryTimeout time.Duration
}

type DispatcherConfig struct {
	QueueSize       int
	DeliveryTimeout time.Duration
}

// NewDispatcher создаёт и запускает асинхронный диспетчер аудита.
func NewDispatcher(publisher *Publisher, cfg DispatcherConfig) *Dispatcher {
	if publisher == nil {
		publisher = NewPublisher()
	}

	dispatcher := &Dispatcher{
		publisher:       publisher,
		events:          make(chan Event, cfg.QueueSize),
		done:            make(chan struct{}),
		deliveryTimeout: cfg.DeliveryTimeout,
	}

	dispatcher.wg.Add(1)
	go dispatcher.run()

	return dispatcher
}

// Notify ставит событие в очередь фоновой доставки.
// Переданный контекст намеренно не используется, чтобы разрыв клиентского
// соединения не отменял аудит уже успешной бизнес-операции.
func (d *Dispatcher) Notify(_ context.Context, event Event) error {
	if d == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return ErrDispatcherClosed
	}

	select {
	case d.events <- event:
		return nil
	case <-d.done:
		return ErrDispatcherClosed
	}
}

// Close прекращает приём новых событий и ждёт доставки уже поставленных в очередь.
func (d *Dispatcher) Close(ctx context.Context) error {
	if d == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	d.closeOnce.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		d.closed = true
		close(d.done)
	})

	closed := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(closed)
	}()

	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Dispatcher) run() {
	defer d.wg.Done()

	for {
		select {
		case event := <-d.events:
			d.deliver(event)
		case <-d.done:
			d.drain()
			return
		}
	}
}

func (d *Dispatcher) drain() {
	for {
		select {
		case event := <-d.events:
			d.deliver(event)
		default:
			return
		}
	}
}

func (d *Dispatcher) deliver(event Event) {
	ctx, cancel := context.WithTimeout(context.Background(), d.deliveryTimeout)
	defer cancel()

	if err := d.publisher.Notify(ctx, event); err != nil {
		log.Error().Err(err).Msg("failed to deliver audit event")
	}
}
