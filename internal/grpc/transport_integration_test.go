package grpcserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/auth"
	grpcserver "github.com/timaogurtzova/shortener/internal/grpc"
	httphandler "github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
	shortenerpb "github.com/timaogurtzova/shortener/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type shortenHTTPResponse struct {
	Result string `json:"result"`
}

type userURLHTTPResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func TestHTTPAndGRPCShareServiceAndAuthorization(t *testing.T) {
	const baseURL = "http://short.local"

	shortener := service.NewShortenerService(context.Background(), repository.NewInMemoryStore())
	t.Cleanup(func() {
		require.NoError(t, shortener.Close(context.Background()))
	})

	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	createHandler := httphandler.NewCreateHandler(shortener, baseURL, authenticator)
	userHandler := httphandler.NewUserHandler(shortener, baseURL, authenticator)
	redirectHandler := httphandler.NewRedirectHandler(shortener)

	router := chi.NewRouter()
	router.Post("/api/shorten", createHandler.CreateShortURLJSON)
	router.Get("/api/user/urls", userHandler.GetUserURLs)
	router.Get("/{id}", redirectHandler.Redirect)

	httpServer := httptest.NewServer(router)
	t.Cleanup(httpServer.Close)

	grpcHandler, err := grpcserver.NewHandler(shortener, baseURL, authenticator, nil)
	require.NoError(t, err)
	grpcClient := newTestClientForHandler(t, grpcHandler)

	httpShortURL, authorizationCookie := createURLOverHTTP(
		t,
		httpServer.Client(),
		httpServer.URL,
		"https://created-over-http.example/path",
	)

	authorizedContext := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", authorizationCookie.Value),
	)
	grpcList, err := grpcClient.ListUserURLs(authorizedContext, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, grpcList.GetUrl(), 1)
	assert.Equal(t, httpShortURL, grpcList.GetUrl()[0].GetShortUrl())

	grpcShorten, err := grpcClient.ShortenURL(
		authorizedContext,
		&shortenerpb.URLShortenRequest{Url: "https://created-over-grpc.example/path"},
	)
	require.NoError(t, err)

	grpcShortID := shortIDFromURL(t, grpcShorten.GetResult())
	noRedirectClient := *httpServer.Client()
	noRedirectClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	redirectResponse, err := noRedirectClient.Get(httpServer.URL + "/" + grpcShortID)
	require.NoError(t, err)
	defer redirectResponse.Body.Close()
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResponse.StatusCode)
	assert.Equal(t, "https://created-over-grpc.example/path", redirectResponse.Header.Get("Location"))

	historyRequest, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/user/urls", nil)
	require.NoError(t, err)
	historyRequest.AddCookie(authorizationCookie)
	historyResponse, err := httpServer.Client().Do(historyRequest)
	require.NoError(t, err)
	defer historyResponse.Body.Close()
	require.Equal(t, http.StatusOK, historyResponse.StatusCode)

	var history []userURLHTTPResponse
	require.NoError(t, json.NewDecoder(historyResponse.Body).Decode(&history))
	assert.ElementsMatch(t, []userURLHTTPResponse{
		{
			ShortURL:    httpShortURL,
			OriginalURL: "https://created-over-http.example/path",
		},
		{
			ShortURL:    grpcShorten.GetResult(),
			OriginalURL: "https://created-over-grpc.example/path",
		},
	}, history)
}

func createURLOverHTTP(
	t *testing.T,
	client *http.Client,
	serverURL string,
	originalURL string,
) (string, *http.Cookie) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"url": originalURL})
	require.NoError(t, err)
	request, err := http.NewRequest(
		http.MethodPost,
		serverURL+"/api/shorten",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusCreated, response.StatusCode)

	var responseBody shortenHTTPResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&responseBody))

	cookies := response.Cookies()
	require.NotEmpty(t, cookies)
	return responseBody.Result, cookies[0]
}

func shortIDFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	shortID := path.Base(parsedURL.Path)
	require.NotEmpty(t, shortID)
	return shortID
}
