package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/audit"
)

func TestFileObserverAppendsEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "audit.log")
	observer, err := audit.NewFileObserver(path)
	require.NoError(t, err)

	first := audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		UserID:    "user-1",
		URL:       "https://example.com/one",
	}
	second := audit.Event{
		Timestamp: 124,
		Action:    audit.ActionFollow,
		UserID:    "user-2",
		URL:       "https://example.com/two",
	}

	require.NoError(t, observer.Update(context.Background(), first))
	require.NoError(t, observer.Update(context.Background(), second))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)

	var gotFirst audit.Event
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &gotFirst))
	assert.Equal(t, first, gotFirst)

	var gotSecond audit.Event
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &gotSecond))
	assert.Equal(t, second, gotSecond)
}

func TestHTTPObserverPostsEvent(t *testing.T) {
	want := audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		UserID:    "user-1",
		URL:       "https://example.com",
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var got audit.Event
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, want, got)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	observer, err := audit.NewHTTPObserver(server.URL)
	require.NoError(t, err)

	require.NoError(t, observer.Update(context.Background(), want))
	assert.Equal(t, 1, requests)
}

func TestHTTPObserverRejectsNonHTTPURL(t *testing.T) {
	tests := []string{
		"ftp://example.com/audit",
		"mailto:audit@example.com",
		"//example.com/audit",
	}

	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			observer, err := audit.NewHTTPObserver(rawURL)
			require.Error(t, err)
			assert.Nil(t, observer)
		})
	}
}

func TestPublisherNotifiesEveryObserver(t *testing.T) {
	publisher := audit.NewPublisher()
	first := &recordingObserver{id: "first"}
	second := &recordingObserver{id: "second"}
	publisher.Register(first)
	publisher.Register(second)

	event := audit.Event{
		Timestamp: 123,
		Action:    audit.ActionFollow,
		URL:       "https://example.com",
	}

	require.NoError(t, publisher.Notify(context.Background(), event))

	assert.Equal(t, []audit.Event{event}, first.events)
	assert.Equal(t, []audit.Event{event}, second.events)
}

func TestPublisherContinuesAfterObserverError(t *testing.T) {
	publisher := audit.NewPublisher()
	failed := &failingObserver{id: "failed"}
	recorded := &recordingObserver{id: "recorded"}
	publisher.Register(failed)
	publisher.Register(recorded)

	event := audit.Event{
		Timestamp: 123,
		Action:    audit.ActionFollow,
		URL:       "https://example.com",
	}

	err := publisher.Notify(context.Background(), event)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
	assert.Equal(t, []audit.Event{event}, recorded.events)
}

func TestDispatcherIgnoresCanceledNotifyContext(t *testing.T) {
	publisher := audit.NewPublisher()
	observer := &channelObserver{
		id:     "recorded",
		events: make(chan audit.Event, 1),
	}
	publisher.Register(observer)

	dispatcher := audit.NewDispatcher(publisher, audit.DispatcherConfig{
		QueueSize:       1,
		DeliveryTimeout: time.Second,
	})
	defer closeDispatcher(t, dispatcher)

	event := audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		UserID:    "user-1",
		URL:       "https://example.com",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, dispatcher.Notify(ctx, event))

	select {
	case got := <-observer.events:
		assert.Equal(t, event, got)
	case <-time.After(time.Second):
		t.Fatal("audit event was not delivered")
	}
}

func TestDispatcherDoesNotBlockOnSlowObserver(t *testing.T) {
	publisher := audit.NewPublisher()
	observer := &slowObserver{
		id:      "slow",
		started: make(chan struct{}),
		delay:   200 * time.Millisecond,
	}
	publisher.Register(observer)

	dispatcher := audit.NewDispatcher(publisher, audit.DispatcherConfig{
		QueueSize:       1,
		DeliveryTimeout: 20 * time.Millisecond,
	})
	defer closeDispatcher(t, dispatcher)

	startedAt := time.Now()
	require.NoError(t, dispatcher.Notify(context.Background(), audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		URL:       "https://example.com",
	}))
	assert.Less(t, time.Since(startedAt), 50*time.Millisecond)

	select {
	case <-observer.started:
	case <-time.After(time.Second):
		t.Fatal("slow observer was not called")
	}
}

func TestDispatcherWaitsUntilQueueHasSpace(t *testing.T) {
	publisher := audit.NewPublisher()
	observer := &blockingObserver{
		id:      "blocking",
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	publisher.Register(observer)

	dispatcher := audit.NewDispatcher(publisher, audit.DispatcherConfig{
		QueueSize:       1,
		DeliveryTimeout: time.Second,
	})
	defer closeDispatcher(t, dispatcher)

	require.NoError(t, dispatcher.Notify(context.Background(), audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		URL:       "https://example.com/one",
	}))

	select {
	case <-observer.started:
	case <-time.After(time.Second):
		t.Fatal("blocking observer was not called")
	}

	require.NoError(t, dispatcher.Notify(context.Background(), audit.Event{
		Timestamp: 124,
		Action:    audit.ActionShorten,
		URL:       "https://example.com/two",
	}))

	enqueued := make(chan error, 1)
	go func() {
		enqueued <- dispatcher.Notify(context.Background(), audit.Event{
			Timestamp: 125,
			Action:    audit.ActionShorten,
			URL:       "https://example.com/three",
		})
	}()

	select {
	case err := <-enqueued:
		t.Fatalf("event was enqueued before queue had space: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(observer.release)

	select {
	case err := <-enqueued:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("event was not enqueued after queue had space")
	}
}

func TestDispatcherRejectsEventsAfterClose(t *testing.T) {
	dispatcher := audit.NewDispatcher(audit.NewPublisher(), audit.DispatcherConfig{
		QueueSize:       1,
		DeliveryTimeout: time.Second,
	})
	closeDispatcher(t, dispatcher)

	err := dispatcher.Notify(context.Background(), audit.Event{
		Timestamp: 123,
		Action:    audit.ActionShorten,
		URL:       "https://example.com",
	})

	assert.ErrorIs(t, err, audit.ErrDispatcherClosed)
}

type recordingObserver struct {
	id     string
	events []audit.Event
}

func (o *recordingObserver) ID() string {
	return o.id
}

func (o *recordingObserver) Update(_ context.Context, event audit.Event) error {
	o.events = append(o.events, event)
	return nil
}

type failingObserver struct {
	id string
}

func (o *failingObserver) ID() string {
	return o.id
}

func (o *failingObserver) Update(context.Context, audit.Event) error {
	return errors.New("observer failed")
}

type channelObserver struct {
	id     string
	events chan audit.Event
}

func (o *channelObserver) ID() string {
	return o.id
}

func (o *channelObserver) Update(ctx context.Context, event audit.Event) error {
	select {
	case o.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type slowObserver struct {
	id      string
	started chan struct{}
	delay   time.Duration
}

func (o *slowObserver) ID() string {
	return o.id
}

func (o *slowObserver) Update(ctx context.Context, event audit.Event) error {
	close(o.started)

	select {
	case <-time.After(o.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type blockingObserver struct {
	id      string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (o *blockingObserver) ID() string {
	return o.id
}

func (o *blockingObserver) Update(ctx context.Context, event audit.Event) error {
	o.once.Do(func() {
		close(o.started)
	})

	select {
	case <-o.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func closeDispatcher(t *testing.T, dispatcher *audit.Dispatcher) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, dispatcher.Close(ctx))
}
