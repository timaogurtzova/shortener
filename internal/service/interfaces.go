package service

// URLShortener описывает контракт, который нужен handler’ам
type URLShortener interface {
	Create(string) (string, error)
	Resolve(string) (string, error)
}
