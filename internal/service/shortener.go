package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"sync"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ShortenerService хранит mapping id → URL
type ShortenerService struct {
	store sync.Map // потокобезопасная map
}

// NewShortenerService создаёт сервис
func NewShortenerService() *ShortenerService {
	return &ShortenerService{}
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

// Create сохраняет URL и возвращает сгенерированный ID
func (s *ShortenerService) Create(url string) (string, error) {
	for {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		// Проверяем коллизию
		_, exists := s.store.Load(id)
		if !exists {
			s.store.Store(id, url)
			return id, nil
		}
		// если коллизия, повторяем генерацию
	}
}

// Resolve возвращает оригинальный URL по ID
func (s *ShortenerService) Resolve(id string) (string, error) {
	value, ok := s.store.Load(id)
	if !ok {
		return "", errors.New("not found")
	}
	return value.(string), nil
}
