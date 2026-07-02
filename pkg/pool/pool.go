// Package pool предоставляет generic-обёртку над sync.Pool для объектов,
// которые умеют очищать своё состояние перед повторным использованием.
package pool

import (
	"reflect"
	"sync"
)

// Resetter описывает объект, состояние которого можно сбросить.
type Resetter interface {
	Reset()
}

// Pool хранит объекты одного типа и сбрасывает их перед возвратом в пул.
// Pool нельзя копировать после первого использования.
type Pool[T Resetter] struct {
	items sync.Pool
}

// New создаёт Pool с функцией создания нового объекта.
// Функция создания может вызываться конкурентно.
func New[T Resetter](newObject func() T) *Pool[T] {
	if newObject == nil {
		panic("pool: new object function is nil")
	}

	return &Pool[T]{
		items: sync.Pool{
			New: func() any {
				object := newObject()
				if isNil(object) {
					panic("pool: new object function returned nil")
				}

				return object
			},
		},
	}
}

// Get возвращает объект из пула или создаёт новый, если пул пуст.
func (p *Pool[T]) Get() T {
	object, ok := p.items.Get().(T)
	if !ok {
		panic("pool: unexpected object type")
	}

	return object
}

// Put сбрасывает состояние объекта и помещает его в пул.
func (p *Pool[T]) Put(object T) {
	if isNil(object) {
		return
	}

	object.Reset()
	p.items.Put(object)
}

func isNil[T any](object T) bool {
	value := reflect.ValueOf(object)
	if !value.IsValid() {
		return true
	}

	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
