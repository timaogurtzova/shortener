package service

import (
	"errors"
	"strings"
)

// MaxOriginalURLLength задаёт максимальный размер исходного URL в байтах.
const MaxOriginalURLLength = 2048

// ErrURLInvalid возвращается, когда исходный URL пуст.
var ErrURLInvalid = errors.New("url is invalid")

// ErrURLTooLong возвращается, когда исходный URL превышает MaxOriginalURLLength.
var ErrURLTooLong = errors.New("url is too long")

// NormalizeOriginalURL нормализует и проверяет исходный URL одинаково для всех транспортов.
func NormalizeOriginalURL(rawURL string) (string, error) {
	originalURL := strings.TrimSpace(rawURL)
	if originalURL == "" {
		return "", ErrURLInvalid
	}
	if len(originalURL) > MaxOriginalURLLength {
		return "", ErrURLTooLong
	}

	return originalURL, nil
}
