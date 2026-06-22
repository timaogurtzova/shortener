package handler

import "net/http"

// userAuthenticator описывает минимальный контракт аутентификатора для HTTP-обработчиков.
type userAuthenticator interface {
	EnsureUserID(w http.ResponseWriter, r *http.Request) (string, error)
	UserIDForHistory(w http.ResponseWriter, r *http.Request) (string, error)
}

// userIDResolver возвращает существующий user ID без создания новой cookie.
type userIDResolver interface {
	UserID(r *http.Request) (string, bool, error)
}
