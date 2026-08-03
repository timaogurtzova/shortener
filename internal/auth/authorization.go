package auth

import (
	"errors"
	"strings"
)

// ErrAuthorizationInvalid возвращается для некорректных авторизационных данных.
var ErrAuthorizationInvalid = errors.New("authorization is invalid")

// EnsureAuthorization возвращает существующий user ID из authorization либо
// создаёт нового пользователя и возвращает новое подписанное значение.
func (a *Authenticator) EnsureAuthorization(authorization string) (userID, newAuthorization string, err error) {
	userID, state := a.resolveAuthorization(authorization)
	if state == credentialStateValid {
		return userID, "", nil
	}

	return a.newAuthorization()
}

// AuthorizationForHistory возвращает user ID для запроса истории.
// При отсутствии authorization создаёт нового пользователя, а некорректные
// данные отклоняет.
func (a *Authenticator) AuthorizationForHistory(authorization string) (userID, newAuthorization string, err error) {
	userID, state := a.resolveAuthorization(authorization)

	switch state {
	case credentialStateValid:
		return userID, "", nil
	case credentialStateMissing:
		return a.newAuthorization()
	default:
		return "", "", ErrAuthorizationInvalid
	}
}

// AuthorizationUserID возвращает существующий валидный user ID, не создавая нового.
func (a *Authenticator) AuthorizationUserID(authorization string) (string, bool) {
	userID, state := a.resolveAuthorization(authorization)
	return userID, state == credentialStateValid
}

func (a *Authenticator) resolveAuthorization(authorization string) (string, credentialState) {
	fields := strings.Fields(authorization)
	if len(fields) == 0 {
		return "", credentialStateMissing
	}

	var token string
	switch {
	case len(fields) == 1:
		token = fields[0]
	case len(fields) == 2 && strings.EqualFold(fields[0], "Bearer"):
		token = fields[1]
	default:
		return "", credentialStateInvalid
	}

	return a.decode(token)
}

func (a *Authenticator) newAuthorization() (userID, authorization string, err error) {
	userID, err = generateUserID(16)
	if err != nil {
		return "", "", err
	}

	authorization, err = a.encode(userID)
	if err != nil {
		return "", "", err
	}

	return userID, authorization, nil
}
