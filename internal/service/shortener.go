package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/timaogurtzova/shortener/internal/repository"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ShortenerService хранит mapping id → URL
type ShortenerService struct {
	repo repository.URLRepository
}

// NewShortenerService создаёт сервис
func NewShortenerService(repo repository.URLRepository) *ShortenerService {
	return &ShortenerService{repo: repo}
}

// Create сохраняет URL и возвращает сгенерированный ID
func (s *ShortenerService) Create(url string) (string, error) {
	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		if err := s.repo.Store(id, url); err == nil {
			return id, nil
		}
	}
	return "", errors.New("cannot generate unique id after 10 attempts")
}

// Resolve возвращает оригинальный URL по ID
func (s *ShortenerService) Resolve(id string) (string, error) {
	return s.repo.Load(id)
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
