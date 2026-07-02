package pool_test

import (
	"testing"

	resetpool "github.com/timaogurtzova/shortener/pkg/pool"
)

type testObject struct {
	values     []string
	labels     map[string]string
	resetCount int
}

func newTestObject() *testObject {
	return &testObject{
		values: make([]string, 0, 4),
		labels: make(map[string]string),
	}
}

func (o *testObject) Reset() {
	o.resetCount++
	o.values = o.values[:0]
	clear(o.labels)
}

func TestPoolGetCreatesObject(t *testing.T) {
	var created int
	pool := resetpool.New(func() *testObject {
		created++
		return newTestObject()
	})

	object := pool.Get()

	if object == nil {
		t.Fatal("Get returned nil object")
	}
	if created != 1 {
		t.Fatalf("constructor should be called once, got %d", created)
	}
	if object.labels == nil {
		t.Fatal("constructor should prepare object fields")
	}
}

func TestPoolPutResetsObject(t *testing.T) {
	pool := resetpool.New(newTestObject)
	object := pool.Get()
	object.values = append(object.values, "first", "second")
	object.labels["key"] = "value"

	pool.Put(object)

	if object.resetCount != 1 {
		t.Fatalf("Reset should be called once, got %d", object.resetCount)
	}
	if len(object.values) != 0 {
		t.Fatalf("slice should be truncated, got len %d", len(object.values))
	}
	if cap(object.values) != 4 {
		t.Fatalf("slice capacity should be preserved, got cap %d", cap(object.values))
	}
	if len(object.labels) != 0 {
		t.Fatalf("map should be cleared, got len %d", len(object.labels))
	}
}

func TestPoolPutIgnoresNilObject(t *testing.T) {
	pool := resetpool.New(newTestObject)

	var object *testObject
	pool.Put(object)

	got := pool.Get()
	if got == nil {
		t.Fatal("Get returned nil after nil object was ignored")
	}
	if got.resetCount != 0 {
		t.Fatalf("nil object should not be reset, got reset count %d", got.resetCount)
	}
}

func TestNewPanicsOnNilConstructor(t *testing.T) {
	assertPanic(t, func() {
		_ = resetpool.New[*testObject](nil)
	})
}

func TestGetPanicsWhenConstructorReturnsNil(t *testing.T) {
	pool := resetpool.New(func() *testObject {
		return nil
	})

	assertPanic(t, func() {
		_ = pool.Get()
	})
}

func assertPanic(t *testing.T, action func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	action()
}
