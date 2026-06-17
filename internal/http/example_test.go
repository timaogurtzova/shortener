package httpserver_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	httpserver "github.com/timaogurtzova/shortener/internal/http"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/model"
)

func ExampleNewRouter_shortenPlainText() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/long"))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(response.Header().Get("Content-Type"))
	fmt.Println(response.Body.String())

	// Output:
	// 201
	// text/plain
	// http://short.local/abc12345
}

func ExampleNewRouter_shortenJSON() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://example.com/json"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(response.Header().Get("Content-Type"))
	fmt.Println(strings.TrimSpace(response.Body.String()))

	// Output:
	// 201
	// application/json
	// {"result":"http://short.local/abc12345"}
}

func ExampleNewRouter_shortenBatch() {
	router := newExampleRouter()

	requestBody := `[
		{"correlation_id":"first","original_url":"https://example.com/first"},
		{"correlation_id":"second","original_url":"https://example.com/second"}
	]`
	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(strings.TrimSpace(response.Body.String()))

	// Output:
	// 201
	// [{"correlation_id":"first","short_url":"http://short.local/batch01"},{"correlation_id":"second","short_url":"http://short.local/batch02"}]
}

func ExampleNewRouter_redirect() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodGet, "/abc12345", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(response.Header().Get("Location"))

	// Output:
	// 307
	// https://example.com/long
}

func ExampleNewRouter_userURLs() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(strings.TrimSpace(response.Body.String()))

	// Output:
	// 200
	// [{"short_url":"http://short.local/abc12345","original_url":"https://example.com/long"}]
}

func ExampleNewRouter_deleteUserURLs() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(`["abc12345"]`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)

	// Output:
	// 202
}

func ExampleNewRouter_ping() {
	router := newExampleRouter()

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)

	// Output:
	// 200
}

func newExampleRouter() http.Handler {
	shortener := newExampleShortener()
	authenticator := exampleAuthenticator{}

	createHandler := handler.NewCreateHandler(shortener, "http://short.local", authenticator)
	userHandler := handler.NewUserHandler(shortener, "http://short.local", authenticator)
	redirectHandler := handler.NewRedirectHandler(shortener)
	redirectHandler.SetUserIDResolver(authenticator)
	pingHandler := handler.NewPingHandler(exampleHealthChecker{})

	return httpserver.NewRouter(httpserver.RouterHandlers{
		CreateShortURLPlainText: createHandler.CreateShortURLPlainText,
		CreateShortURLJSON:      createHandler.CreateShortURLJSON,
		CreateShortURLBatchJSON: createHandler.CreateShortURLBatchJSON,
		GetUserURLs:             userHandler.GetUserURLs,
		DeleteUserURLs:          userHandler.DeleteUserURLs,
		Redirect:                redirectHandler.Redirect,
		Ping:                    pingHandler.Ping,
	})
}

type exampleShortener struct {
	urls     map[string]string
	userURLs []model.UserURL
}

func newExampleShortener() *exampleShortener {
	return &exampleShortener{
		urls: map[string]string{
			"abc12345": "https://example.com/long",
		},
		userURLs: []model.UserURL{
			{
				ShortID:     "abc12345",
				OriginalURL: "https://example.com/long",
			},
		},
	}
}

func (s *exampleShortener) Create(_ context.Context, url, _ string) (string, error) {
	s.urls["abc12345"] = url
	return "abc12345", nil
}

func (s *exampleShortener) CreateBatch(_ context.Context, urls []string, _ string) ([]string, error) {
	ids := make([]string, len(urls))
	for i := range urls {
		ids[i] = fmt.Sprintf("batch%02d", i+1)
	}

	return ids, nil
}

func (s *exampleShortener) Resolve(_ context.Context, id string) (string, error) {
	url, ok := s.urls[id]
	if !ok {
		return "", errors.New("url not found")
	}

	return url, nil
}

func (s *exampleShortener) FindByUserID(context.Context, string) ([]model.UserURL, error) {
	return append([]model.UserURL(nil), s.userURLs...), nil
}

func (s *exampleShortener) DeleteUserURLs(context.Context, string, []string) error {
	return nil
}

type exampleAuthenticator struct{}

func (exampleAuthenticator) EnsureUserID(http.ResponseWriter, *http.Request) (string, error) {
	return "user-1", nil
}

func (exampleAuthenticator) UserIDForHistory(http.ResponseWriter, *http.Request) (string, error) {
	return "user-1", nil
}

func (exampleAuthenticator) UserID(*http.Request) (string, bool, error) {
	return "user-1", true, nil
}

type exampleHealthChecker struct{}

func (exampleHealthChecker) Ping(context.Context) error {
	return nil
}
