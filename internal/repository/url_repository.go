package repository

// URLRepository описывает контракт хранилища URL
type URLRepository interface {
	Store(id, url string) error
	Load(id string) (string, error)
}
