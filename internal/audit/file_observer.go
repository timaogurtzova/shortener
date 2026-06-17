package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const fileObserverID = "audit-file"

// FileObserver добавляет события аудита в файл формата JSON Lines.
type FileObserver struct {
	mu   sync.Mutex
	path string
}

// NewFileObserver создаёт файловый наблюдатель аудита.
func NewFileObserver(path string) (*FileObserver, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("audit file path is empty")
	}

	return &FileObserver{path: path}, nil
}

// ID возвращает стабильный идентификатор наблюдателя.
func (o *FileObserver) ID() string {
	return fileObserverID
}

// Update добавляет одно событие аудита в настроенный файл.
func (o *FileObserver) Update(ctx context.Context, event Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()

	dir := filepath.Dir(o.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(o.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(line)
	return err
}
