package audit

import "time"

// Action описывает действие пользователя для аудита.
type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

// Event хранит одно событие аудита.
type Event struct {
	Timestamp int64  `json:"ts"`
	Action    Action `json:"action"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
}

// NewEvent создаёт событие аудита с текущим Unix timestamp.
func NewEvent(action Action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}
