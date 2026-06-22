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
	file *os.File
}

// NewFileObserver создаёт файловый наблюдатель аудита.
func NewFileObserver(path string) (*FileObserver, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("audit file path is empty")
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	return &FileObserver{file: file}, nil
}

// ID возвращает стабильный идентификатор наблюдателя.
func (o *FileObserver) ID() string {
	return fileObserverID
}

// Update добавляет одно событие аудита в настроенный файл.
func (o *FileObserver) Update(ctx context.Context, event Event) error {
	if o == nil {
		return nil
	}
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

	if o.file == nil {
		return errors.New("audit file observer is closed")
	}

	_, err = o.file.Write(line)
	return err
}

// Close закрывает файл-приёмник аудита.
func (o *FileObserver) Close() error {
	if o == nil {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return nil
	}

	err := o.file.Close()
	o.file = nil
	return err
}
