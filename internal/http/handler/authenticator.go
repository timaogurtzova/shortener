package handler

import "net/http"

// userAuthenticator описывает минимальный контракт аутентификатора для HTTP-обработчиков.
type userAuthenticator interface {
	EnsureUserID(w http.ResponseWriter, r *http.Request) (string, error)
	UserIDForHistory(w http.ResponseWriter, r *http.Request) (string, error)
}
