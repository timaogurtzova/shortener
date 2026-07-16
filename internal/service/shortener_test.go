package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestShortenerService_Create(t *testing.T) {
	tests := []struct {
		name          string
		mockStoreFunc func(ctx context.Context, id, url, userID string) error
		wantID        string
		wantErr       bool
		wantConflict  bool
	}{
		{
			name: "успешное создание",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				assert.Equal(t, "user-1", userID)
				return nil
			},
			wantErr:      false,
			wantConflict: false,
		},
		{
			name: "ошибка при сохранении",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				return errors.New("store error")
			},
			wantErr:      true,
			wantConflict: false,
		},
		{
			name: "исходный url уже сокращён",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				return &repository.URLConflictError{ShortID: "abc123"}
			},
			wantID:       "abc123",
			wantErr:      true,
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				storeFunc: tt.mockStoreFunc,
			}
			svc := service.NewShortenerService(context.Background(), mockRepo)

			id, err := svc.Create(context.Background(), "https://example.com", "user-1")
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantConflict {
					assert.ErrorIs(t, err, service.ErrURLAlreadyExists)
					assert.Equal(t, tt.wantID, id)
				}
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, id)
			}
		})
	}
}

func TestShortenerService_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		mockLoad func(ctx context.Context, id string) (string, error)
		wantURL  string
		wantErr  bool
	}{
		{
			name: "успешное получение",
			mockLoad: func(ctx context.Context, id string) (string, error) {
				return "https://example.com", nil
			},
			wantURL: "https://example.com",
			wantErr: false,
		},
		{
			name: "id не найден",
			mockLoad: func(ctx context.Context, id string) (string, error) {
				return "", errors.New("not found")
			},
			wantURL: "",
			wantErr: true,
		},
		{
			name: "id удалён",
			mockLoad: func(ctx context.Context, id string) (string, error) {
				return "", repository.ErrDeleted
			},
			wantURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				loadFunc: tt.mockLoad,
			}
			svc := service.NewShortenerService(context.Background(), mockRepo)

			url, err := svc.Resolve(context.Background(), "abc123")
			if tt.wantErr {
				assert.Error(t, err)
				if tt.name == "id удалён" {
					assert.ErrorIs(t, err, service.ErrURLDeleted)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantURL, url)
			}
		})
	}
}

func TestShortenerService_CreateBatch(t *testing.T) {
	tests := []struct {
		name               string
		urls               []string
		mockBatchStoreFunc func(ctx context.Context, records []repository.BatchRecord) error
		wantErr            bool
		wantLen            int
	}{
		{
			name: "успешное пакетное создание",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(ctx context.Context, records []repository.BatchRecord) error {
				return nil
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "ошибка пакетного сохранения",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(ctx context.Context, records []repository.BatchRecord) error {
				return errors.New("batch store error")
			},
			wantErr: true,
			wantLen: 0,
		},
		{
			name:    "пустой батч",
			urls:    nil,
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				batchStoreFunc: tt.mockBatchStoreFunc,
			}
			svc := service.NewShortenerService(context.Background(), mockRepo)

			ids, err := svc.CreateBatch(context.Background(), tt.urls, "user-1")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, ids, tt.wantLen)
			for _, id := range ids {
				assert.NotEmpty(t, id)
			}
		})
	}
}

func TestShortenerService_FindByUserID(t *testing.T) {
	expected := []model.UserURL{
		{ShortID: "abc123", OriginalURL: "https://example.com"},
	}

	mockRepo := &mockURLRepository{
		findByUserFunc: func(ctx context.Context, userID string) ([]model.UserURL, error) {
			assert.Equal(t, "user-1", userID)
			return expected, nil
		},
	}

	svc := service.NewShortenerService(context.Background(), mockRepo)

	actual, err := svc.FindByUserID(context.Background(), "user-1")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestShortenerService_DeleteUserURLs(t *testing.T) {
	called := make(chan []string, 1)

	mockRepo := &mockURLRepository{
		markDeletedFunc: func(ctx context.Context, userID string, shortIDs []string) error {
			assert.Equal(t, "user-1", userID)
			called <- shortIDs
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := service.NewShortenerService(ctx, mockRepo)
	require.NoError(t, svc.DeleteUserURLs(context.Background(), "user-1", []string{"abc123", "def456", "abc123"}))

	select {
	case shortIDs := <-called:
		assert.ElementsMatch(t, []string{"abc123", "def456"}, shortIDs)
	case <-time.After(time.Second):
		t.Fatal("delete request was not processed asynchronously")
	}
}

func TestShortenerServiceCloseFlushesQueuedDeletions(t *testing.T) {
	deletedByUser := make(map[string][]string)
	mockRepo := &mockURLRepository{
		markDeletedFunc: func(_ context.Context, userID string, shortIDs []string) error {
			deletedByUser[userID] = append(deletedByUser[userID], shortIDs...)
			return nil
		},
	}

	svc := service.NewShortenerService(context.Background(), mockRepo)
	require.NoError(t, svc.DeleteUserURLs(context.Background(), "user-1", []string{"abc123", "def456"}))
	require.NoError(t, svc.DeleteUserURLs(context.Background(), "user-2", []string{"ghi789"}))

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, svc.Close(closeCtx))

	assert.ElementsMatch(t, []string{"abc123", "def456"}, deletedByUser["user-1"])
	assert.ElementsMatch(t, []string{"ghi789"}, deletedByUser["user-2"])
	assert.NoError(t, svc.Close(context.Background()), "Close must be idempotent")
}

func TestShortenerServiceCloseReturnsFlushError(t *testing.T) {
	wantErr := errors.New("persist deletion")
	mockRepo := &mockURLRepository{
		markDeletedFunc: func(_ context.Context, _ string, _ []string) error {
			return wantErr
		},
	}

	svc := service.NewShortenerService(context.Background(), mockRepo)
	require.NoError(t, svc.DeleteUserURLs(context.Background(), "user-1", []string{"abc123"}))

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := svc.Close(closeCtx)

	assert.ErrorIs(t, err, wantErr)
}

var (
	benchmarkID  string
	benchmarkIDs []string
	benchmarkURL string
)

func BenchmarkGenerateID(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		id, err := service.GenerateID(8)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkID = id
	}
}

func BenchmarkShortenerServiceCreate(b *testing.B) {
	b.ReportAllocs()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := service.NewShortenerService(ctx, &mockURLRepository{
		storeFunc: func(ctx context.Context, id, url, userID string) error {
			return nil
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, err := svc.Create(ctx, "https://example.com/articles/benchmark", "user-1")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkID = id
	}
}

func BenchmarkShortenerServiceCreateBatch(b *testing.B) {
	b.ReportAllocs()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urls := make([]string, 100)
	for i := range urls {
		urls[i] = "https://example.com/articles/benchmark"
	}

	svc := service.NewShortenerService(ctx, &mockURLRepository{
		batchStoreFunc: func(ctx context.Context, records []repository.BatchRecord) error {
			return nil
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ids, err := svc.CreateBatch(ctx, urls, "user-1")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkIDs = ids
	}
}

func BenchmarkShortenerServiceResolve(b *testing.B) {
	b.ReportAllocs()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := service.NewShortenerService(ctx, &mockURLRepository{
		loadFunc: func(ctx context.Context, id string) (string, error) {
			return "https://example.com/articles/benchmark", nil
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url, err := svc.Resolve(ctx, "abc12345")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkURL = url
	}
}
