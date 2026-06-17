package handler

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/audit"
)

type auditNotifier interface {
	Notify(context.Context, audit.Event) error
}

func publishAuditEvent(ctx context.Context, notifier auditNotifier, action audit.Action, userID, originalURL string) {
	if notifier == nil {
		return
	}

	event := audit.NewEvent(action, userID, originalURL)
	if err := notifier.Notify(ctx, event); err != nil {
		log.Warn().Err(err).Msg("failed to enqueue audit event")
	}
}
