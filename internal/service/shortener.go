package service

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"

	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/repository"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const maxGenerateAttempts = 10
const randomByteLimit = byte(256 - 256%len(letters))

// ErrURLAlreadyExists возвращается, когда исходный URL уже был сокращён ранее.
var ErrURLAlreadyExists = errors.New("url already exists")

// ErrURLDeleted возвращается, когда короткий URL помечен как удалённый.
var ErrURLDeleted = errors.New("url deleted")

// ShortenerService реализует бизнес-логику сокращения, поиска и удаления URL.
type ShortenerService struct {
	repo           repository.URLRepository
	deleteQueue    chan deleteRequest
	workerDone     chan struct{}
	workerCancel   context.CancelFunc
	lifecycleMu    sync.Mutex
	enqueueCond    *sync.Cond
	activeEnqueues int
	closed         bool
	closeOnce      sync.Once
	workerErr      error
}

// NewShortenerService создаёт сервис сокращения URL и запускает фоновый воркер удаления.
func NewShortenerService(ctx context.Context, repo repository.URLRepository) *ShortenerService {
	if ctx == nil {
		ctx = context.Background()
	}
	workerCtx, workerCancel := context.WithCancel(ctx)

	svc := &ShortenerService{
		repo:         repo,
		deleteQueue:  make(chan deleteRequest, deleteQueueSize),
		workerDone:   make(chan struct{}),
		workerCancel: workerCancel,
	}
	svc.enqueueCond = sync.NewCond(&svc.lifecycleMu)

	go func() {
		err := svc.runDeleteWorker(workerCtx)
		svc.lifecycleMu.Lock()
		svc.workerErr = err
		svc.lifecycleMu.Unlock()
		close(svc.workerDone)
	}()

	return svc
}

// Create сохраняет исходный URL и возвращает сгенерированный короткий идентификатор.
func (s *ShortenerService) Create(ctx context.Context, url, userID string) (string, error) {
	for i := 0; i < maxGenerateAttempts; i++ {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		err = s.repo.Store(ctx, id, url, userID)
		if err == nil {
			return id, nil
		}

		var conflictErr *repository.URLConflictError
		if errors.As(err, &conflictErr) {
			return conflictErr.ShortID, ErrURLAlreadyExists
		}

		if !errors.Is(err, repository.ErrIDAlreadyExists) {
			return "", err
		}
	}
	return "", errors.New("cannot generate unique id after 10 attempts")
}

// CreateBatch сохраняет пакет URL и возвращает сгенерированные ID в исходном порядке.
func (s *ShortenerService) CreateBatch(ctx context.Context, urls []string, userID string) ([]string, error) {
	if len(urls) == 0 {
		return nil, errors.New("empty batch")
	}

	for i := 0; i < maxGenerateAttempts; i++ {
		records, ids, err := buildBatchRecords(urls, userID)
		if err != nil {
			return nil, err
		}

		err = s.repo.BatchStore(ctx, records)
		if err == nil {
			return ids, nil
		}

		if !errors.Is(err, repository.ErrIDAlreadyExists) {
			return nil, err
		}
	}

	return nil, errors.New("cannot generate unique ids after 10 attempts")
}

// Resolve возвращает исходный URL по короткому идентификатору.
func (s *ShortenerService) Resolve(ctx context.Context, id string) (string, error) {
	originalURL, err := s.repo.Load(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrDeleted) {
			return "", ErrURLDeleted
		}

		return "", err
	}

	return originalURL, nil
}

// FindByUserID возвращает все URL, сокращённые пользователем.
func (s *ShortenerService) FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error) {
	return s.repo.FindByUserID(ctx, userID)
}

// GenerateID создаёт криптографически случайный короткий идентификатор длиной n.
func GenerateID(n int) (string, error) {
	if n < 0 {
		return "", errors.New("invalid id length")
	}
	if n == 0 {
		return "", nil
	}

	result := make([]byte, n)
	randomBytes := make([]byte, n)

	for i := 0; i < n; {
		if _, err := rand.Read(randomBytes); err != nil {
			return "", err
		}

		for _, randomByte := range randomBytes {
			if randomByte >= randomByteLimit {
				continue
			}

			result[i] = letters[int(randomByte)%len(letters)]
			i++
			if i == n {
				break
			}
		}
	}

	return string(result), nil
}

func buildBatchRecords(urls []string, userID string) ([]repository.BatchRecord, []string, error) {
	records := make([]repository.BatchRecord, len(urls))
	ids := make([]string, len(urls))
	usedIDs := make(map[string]struct{}, len(urls))

	for i, originalURL := range urls {
		id, err := generateUniqueID(usedIDs)
		if err != nil {
			return nil, nil, err
		}

		usedIDs[id] = struct{}{}
		ids[i] = id
		records[i] = repository.BatchRecord{
			ID:          id,
			OriginalURL: originalURL,
			UserID:      userID,
		}
	}

	return records, ids, nil
}

func generateUniqueID(usedIDs map[string]struct{}) (string, error) {
	for i := 0; i < maxGenerateAttempts; i++ {
		id, err := GenerateID(8)
		if err != nil {
			return "", err
		}

		if _, exists := usedIDs[id]; !exists {
			return id, nil
		}
	}

	return "", errors.New("cannot generate unique id after 10 attempts")
}
