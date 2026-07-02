package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const httpObserverID = "audit-http"
const defaultHTTPTimeout = 5 * time.Second

// HTTPObserver отправляет события аудита удалённому приёмнику.
type HTTPObserver struct {
	client *http.Client
	url    string
}

// NewHTTPObserver создаёт HTTP-наблюдатель аудита.
func NewHTTPObserver(rawURL string) (*HTTPObserver, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("audit url is empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || !isHTTPURL(parsedURL) {
		return nil, errors.New("invalid audit url")
	}

	return &HTTPObserver{
		client: &http.Client{Timeout: defaultHTTPTimeout},
		url:    rawURL,
	}, nil
}

// ID возвращает стабильный идентификатор наблюдателя.
func (o *HTTPObserver) ID() string {
	return httpObserverID
}

// Update отправляет одно событие аудита настроенному удалённому приёмнику.
func (o *HTTPObserver) Update(ctx context.Context, event Event) error {
	if ctx == nil {
		ctx = context.Background()
	}

	body := jsonBufferPool.Get()
	defer jsonBufferPool.Put(body)

	if err := json.NewEncoder(body).Encode(event); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("audit receiver returned status %d", resp.StatusCode)
	}

	return nil
}

func isHTTPURL(parsedURL *url.URL) bool {
	return parsedURL != nil &&
		(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") &&
		parsedURL.Host != ""
}
