package service

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	deleteQueueSize     = 256
	deleteBatchSize     = 1024
	deleteFlushInterval = 200 * time.Millisecond
	deleteFlushTimeout  = 5 * time.Second
)

type deleteRequest struct {
	userID   string
	shortIDs []string
}

// DeleteUserURLs принимает запрос на асинхронное удаление URL пользователя.
func (s *ShortenerService) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if userID == "" {
		return nil
	}

	request := deleteRequest{
		userID:   userID,
		shortIDs: normalizeShortIDs(shortIDs),
	}
	if len(request.shortIDs) == 0 {
		return nil
	}

	select {
	case s.deleteQueue <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.workerDone:
		return nil
	}

	return nil
}

func (s *ShortenerService) runDeleteWorker(ctx context.Context) {
	defer close(s.workerDone)

	ticker := time.NewTicker(deleteFlushInterval)
	defer ticker.Stop()

	pending := make(map[string]map[string]struct{})
	pendingCount := 0

	flush := func() {
		if pendingCount == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), deleteFlushTimeout)
		defer cancel()

		for userID, shortIDsSet := range pending {
			shortIDs := make([]string, 0, len(shortIDsSet))
			for shortID := range shortIDsSet {
				shortIDs = append(shortIDs, shortID)
			}

			if err := s.repo.MarkDeleted(ctx, userID, shortIDs); err != nil {
				log.Error().
					Err(err).
					Str("user_id", userID).
					Int("short_ids_count", len(shortIDs)).
					Msg("failed to mark urls as deleted")
			}
		}

		pending = make(map[string]map[string]struct{})
		pendingCount = 0
	}

	for {
		select {
		case request := <-s.deleteQueue:
			pendingCount += addDeleteRequest(pending, request)
			if pendingCount >= deleteBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

func addDeleteRequest(pending map[string]map[string]struct{}, request deleteRequest) int {
	if request.userID == "" || len(request.shortIDs) == 0 {
		return 0
	}

	userShortIDs, exists := pending[request.userID]
	if !exists {
		userShortIDs = make(map[string]struct{})
		pending[request.userID] = userShortIDs
	}

	added := 0
	for _, shortID := range request.shortIDs {
		if _, exists := userShortIDs[shortID]; exists {
			continue
		}

		userShortIDs[shortID] = struct{}{}
		added++
	}

	return added
}

func normalizeShortIDs(shortIDs []string) []string {
	result := make([]string, 0, len(shortIDs))
	seen := make(map[string]struct{}, len(shortIDs))

	for _, shortID := range shortIDs {
		shortID = strings.TrimSpace(shortID)
		if shortID == "" {
			continue
		}

		if _, exists := seen[shortID]; exists {
			continue
		}

		seen[shortID] = struct{}{}
		result = append(result, shortID)
	}

	return result
}
