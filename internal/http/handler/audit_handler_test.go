package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/audit"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestCreateShortURLPlainTextPublishesAuditEvent(t *testing.T) {
	notifier := &recordingAuditNotifier{}
	svc := &mockURLShortener{
		CreateMockFunc: func(ctx context.Context, url, userID string) (string, error) {
			assert.Equal(t, "https://example.com", url)
			assert.NotEmpty(t, userID)
			return "abc123", nil
		},
	}

	h := handler.NewCreateHandler(svc, "http://localhost:8080", newTestAuthenticator(t))
	h.SetAuditPublisher(notifier)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	h.CreateShortURLPlainText(rec, req)

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Positive(t, event.Timestamp)
	assert.Equal(t, audit.ActionShorten, event.Action)
	assert.NotEmpty(t, event.UserID)
	assert.Equal(t, "https://example.com", event.URL)
}

func TestCreateShortURLJSONPublishesAuditEvent(t *testing.T) {
	notifier := &recordingAuditNotifier{}
	svc := &mockURLShortener{
		CreateMockFunc: func(ctx context.Context, url, userID string) (string, error) {
			return "abc123", nil
		},
	}

	h := handler.NewCreateHandler(svc, "http://localhost:8080", newTestAuthenticator(t))
	h.SetAuditPublisher(notifier)

	reqBody, err := json.Marshal(map[string]string{"url": "https://example.com/json"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.CreateShortURLJSON(rec, req)

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Positive(t, event.Timestamp)
	assert.Equal(t, audit.ActionShorten, event.Action)
	assert.NotEmpty(t, event.UserID)
	assert.Equal(t, "https://example.com/json", event.URL)
}

func TestRedirectPublishesAuditEvent(t *testing.T) {
	notifier := &recordingAuditNotifier{}
	authenticator := newTestAuthenticator(t)
	cookie, err := authenticator.NewCookie("user-123")
	require.NoError(t, err)

	svc := &mockURLShortener{
		ResolveMockFunc: func(ctx context.Context, id string) (string, error) {
			assert.Equal(t, "abc123", id)
			return "https://example.com/target", nil
		},
	}

	h := handler.NewRedirectHandler(svc)
	h.SetAuditPublisher(notifier)
	h.SetUserIDResolver(authenticator)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.AddCookie(cookie)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Positive(t, event.Timestamp)
	assert.Equal(t, audit.ActionFollow, event.Action)
	assert.Equal(t, "user-123", event.UserID)
	assert.Equal(t, "https://example.com/target", event.URL)
}

func TestCreateShortURLDoesNotPublishAuditOnError(t *testing.T) {
	notifier := &recordingAuditNotifier{}
	svc := &mockURLShortener{
		CreateMockFunc: func(ctx context.Context, url, userID string) (string, error) {
			return "", service.ErrURLDeleted
		},
	}

	h := handler.NewCreateHandler(svc, "http://localhost:8080", newTestAuthenticator(t))
	h.SetAuditPublisher(notifier)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	h.CreateShortURLPlainText(rec, req)

	assert.Empty(t, notifier.events)
}

func TestCreateShortURLDoesNotPublishAuditOnConflict(t *testing.T) {
	notifier := &recordingAuditNotifier{}
	svc := &mockURLShortener{
		CreateMockFunc: func(ctx context.Context, url, userID string) (string, error) {
			return "abc123", service.ErrURLAlreadyExists
		},
	}

	h := handler.NewCreateHandler(svc, "http://localhost:8080", newTestAuthenticator(t))
	h.SetAuditPublisher(notifier)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	h.CreateShortURLPlainText(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Empty(t, notifier.events)
}

func TestCreateShortURLPublishesAuditWithCanceledRequestContext(t *testing.T) {
	publisher := audit.NewPublisher()
	observer := &channelAuditObserver{events: make(chan audit.Event, 1)}
	publisher.Register(observer)
	dispatcher := audit.NewDispatcher(publisher, audit.DispatcherConfig{
		QueueSize:       1,
		DeliveryTimeout: time.Second,
	})
	defer closeAuditDispatcher(t, dispatcher)

	svc := &mockURLShortener{
		CreateMockFunc: func(ctx context.Context, url, userID string) (string, error) {
			return "abc123", nil
		},
	}

	h := handler.NewCreateHandler(svc, "http://localhost:8080", newTestAuthenticator(t))
	h.SetAuditPublisher(dispatcher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com")).WithContext(ctx)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	h.CreateShortURLPlainText(rec, req)

	select {
	case event := <-observer.events:
		assert.Equal(t, audit.ActionShorten, event.Action)
		assert.Equal(t, "https://example.com", event.URL)
	case <-time.After(time.Second):
		t.Fatal("audit event was not delivered")
	}
}

type recordingAuditNotifier struct {
	events []audit.Event
}

func (p *recordingAuditNotifier) Notify(_ context.Context, event audit.Event) error {
	p.events = append(p.events, event)
	return nil
}

type channelAuditObserver struct {
	events chan audit.Event
}

func (o *channelAuditObserver) ID() string {
	return "channel"
}

func (o *channelAuditObserver) Update(ctx context.Context, event audit.Event) error {
	select {
	case o.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func closeAuditDispatcher(t *testing.T, dispatcher *audit.Dispatcher) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, dispatcher.Close(ctx))
}
