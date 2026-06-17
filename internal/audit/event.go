package audit

import "time"

// Action описывает действие пользователя для аудита.
type Action string

// Действия аудита описывают пользовательские операции, которые попадают в аудит.
const (
	// ActionShorten обозначает успешное создание короткого URL.
	ActionShorten Action = "shorten"

	// ActionFollow обозначает успешный переход по короткому URL.
	ActionFollow Action = "follow"
)

// Event хранит одно событие аудита.
type Event struct {
	// Timestamp хранит Unix-время события.
	Timestamp int64 `json:"ts"`

	// Action хранит тип действия пользователя.
	Action Action `json:"action"`

	// UserID хранит идентификатор пользователя, если он известен.
	UserID string `json:"user_id"`

	// URL хранит исходный URL, связанный с событием аудита.
	URL string `json:"url"`
}

// NewEvent создаёт событие аудита с текущим Unix-временем.
func NewEvent(action Action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}
