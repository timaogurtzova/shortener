package model

// UserURL описывает сокращённый пользователем URL.
type UserURL struct {
	// ShortID хранит короткий идентификатор URL.
	ShortID string

	// OriginalURL хранит исходный URL.
	OriginalURL string
}
