package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/timaogurtzova/shortener/internal/repository"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const maxGenerateAttempts = 10

// ErrURLAlreadyExists возвращается, когда исходный URL уже был сокращён ранее.
var ErrURLAlreadyExists = errors.New("url already exists")

// ShortenerService хранит mapping id → URL
type ShortenerService struct {
	repo repository.URLRepository
}

// NewShortenerService создаёт сервис
func NewShortenerService(repo repository.URLRepository) *ShortenerService {
	return &ShortenerService{repo: repo}
}

// Create сохраняет URL и возвращает сгенерированный ID
func (s *ShortenerService) Create(ctx context.Context, url string) (string, error) {
	for i := 0; i < maxGenerateAttempts; i++ {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		err = s.repo.Store(ctx, id, url)
		if err == nil {
			return id, nil
		}

		var conflictErr *repository.URLConflictError
		if errors.As(err, &conflictErr) {
			return conflictErr.ShortID, ErrURLAlreadyExists
		}

		if !errors.Is(err, repository.ErrIDAlreadyExists) {
			return "", err
		}
	}
	return "", errors.New("cannot generate unique id after 10 attempts")
}

// CreateBatch сохраняет пакет URL и возвращает сгенерированные ID в исходном порядке.
func (s *ShortenerService) CreateBatch(ctx context.Context, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return nil, errors.New("empty batch")
	}

	for i := 0; i < maxGenerateAttempts; i++ {
		records, ids, err := buildBatchRecords(urls)
		if err != nil {
			return nil, err
		}

		err = s.repo.BatchStore(ctx, records)
		if err == nil {
			return ids, nil
		}

		if !errors.Is(err, repository.ErrIDAlreadyExists) {
			return nil, err
		}
	}

	return nil, errors.New("cannot generate unique ids after 10 attempts")
}

// Resolve возвращает оригинальный URL по ID
func (s *ShortenerService) Resolve(ctx context.Context, id string) (string, error) {
	return s.repo.Load(ctx, id)
}

// GenerateID создаёт случайный ID длиной n
func GenerateID(n int) (string, error) {
	var result strings.Builder
	result.Grow(n)

	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		result.WriteByte(letters[num.Int64()])
	}

	return result.String(), nil
}

func buildBatchRecords(urls []string) ([]repository.BatchRecord, []string, error) {
	records := make([]repository.BatchRecord, len(urls))
	ids := make([]string, len(urls))
	usedIDs := make(map[string]struct{}, len(urls))

	for i, originalURL := range urls {
		id, err := generateUniqueID(usedIDs)
		if err != nil {
			return nil, nil, err
		}

		usedIDs[id] = struct{}{}
		ids[i] = id
		records[i] = repository.BatchRecord{
			ID:          id,
			OriginalURL: originalURL,
		}
	}

	return records, ids, nil
}

func generateUniqueID(usedIDs map[string]struct{}) (string, error) {
	for i := 0; i < maxGenerateAttempts; i++ {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		if _, exists := usedIDs[id]; !exists {
			return id, nil
		}
	}

	return "", errors.New("cannot generate unique id after 10 attempts")
}
