package model

// Stats содержит агрегированную статистику сервиса сокращения URL.
type Stats struct {
	// URLs содержит количество сокращённых URL.
	URLs int

	// Users содержит количество пользователей, сокращавших URL.
	Users int
}
