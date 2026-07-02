package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sample.go"), `package sample

type Child struct {
	value int
}

func (c *Child) Reset() {
	c.value = 42
}

type Scores []int
type Labels map[string]string
type Name string

// generate:reset
type Target struct {
	i        int
	str      string
	flag     bool
	scores   []int
	labels   map[string]string
	namedS   Scores
	namedM   Labels
	name     Name
	strP     *string
	child    Child
	childP   *Child
	childPP  **Child
	nested   struct {
		values []string
		cache  map[string]int
		count  int
	}
	nestedP *struct {
		count int
	}
}

// generate:reset
type Box[T any] struct {
	value T
	items []T
}
`)

	if err := generate(dir); err != nil {
		t.Fatalf("generate reset methods: %v", err)
	}

	generatedPath := filepath.Join(dir, generatedFileName)
	data, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	source := string(data)

	if _, err := parser.ParseFile(token.NewFileSet(), generatedPath, source, parser.ParseComments); err != nil {
		t.Fatalf("generated file is not valid Go: %v\n%s", err, source)
	}

	assertContains(t, source, "func (v *Box[T]) Reset()")
	assertContains(t, source, "func (v *Target) Reset()")
	assertContains(t, source, "if v == nil")
	assertContains(t, source, "v.i = 0")
	assertContains(t, source, `v.str = ""`)
	assertContains(t, source, "v.flag = false")
	assertContains(t, source, "v.scores = v.scores[:0]")
	assertContains(t, source, "clear(v.labels)")
	assertContains(t, source, "v.namedS = v.namedS[:0]")
	assertContains(t, source, "clear(v.namedM)")
	assertContains(t, source, `v.name = ""`)
	assertContains(t, source, "if v.strP != nil")
	assertContains(t, source, `*v.strP = ""`)
	assertContains(t, source, "v.child.Reset()")
	assertContains(t, source, "v.childP.Reset()")
	assertContains(t, source, "(*v.childPP).Reset()")
	assertContains(t, source, "v.nested.values = v.nested.values[:0]")
	assertContains(t, source, "clear(v.nested.cache)")
	assertContains(t, source, "v.nested.count = 0")
	assertContains(t, source, "(*v.nestedP).count = 0")
}

func TestGeneratedResetBehavior(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module sample

go 1.24
`)
	writeFile(t, filepath.Join(dir, "sample.go"), `package sample

type Scores []int
type Labels map[string]string
type Name string

type Child struct {
	value int
}

func (c *Child) Reset() {
	c.value = 42
}

type WrongReset struct {
	value int
}

func (w *WrongReset) Reset(value int) {
	w.value = value
}

// generate:reset
type Target struct {
	count  int
	name   Name
	scores Scores
	labels Labels
	child  Child
	childP *Child
	wrong  WrongReset
	textP  *string
	nested *struct {
		values []string
		cache  map[string]int
	}
}

// generate:reset
type Box[T any] struct {
	value T
	items []T
}
`)
	writeFile(t, filepath.Join(dir, "sample_test.go"), `package sample

import "testing"

func TestResetBehavior(t *testing.T) {
	text := "filled"
	child := &Child{value: 100}
	target := &Target{
		count:  7,
		name:   "name",
		scores: Scores{1, 2, 3},
		labels: Labels{"a": "b"},
		child:  Child{value: 100},
		childP: child,
		wrong:  WrongReset{value: 100},
		textP:  &text,
		nested: &struct {
			values []string
			cache  map[string]int
		}{
			values: []string{"x", "y"},
			cache:  map[string]int{"x": 1},
		},
	}

	target.Reset()

	if target.count != 0 || target.name != "" || text != "" {
		t.Fatalf("primitive fields were not reset: %#v, text=%q", target, text)
	}
	if len(target.scores) != 0 || cap(target.scores) != 3 {
		t.Fatalf("named slice should be truncated without dropping capacity: len=%d cap=%d", len(target.scores), cap(target.scores))
	}
	if len(target.labels) != 0 || target.labels == nil {
		t.Fatalf("named map should be cleared without nil assignment: %#v", target.labels)
	}
	if target.child.value != 42 || target.childP.value != 42 {
		t.Fatalf("child Reset methods were not called: value=%d pointer=%d", target.child.value, target.childP.value)
	}
	if target.wrong.value != 0 {
		t.Fatalf("Reset method with arguments should not be called: %#v", target.wrong)
	}
	if len(target.nested.values) != 0 || cap(target.nested.values) != 2 || len(target.nested.cache) != 0 {
		t.Fatalf("nested anonymous struct was not reset: %#v", target.nested)
	}

	box := &Box[int]{value: 10, items: []int{1, 2, 3}}
	box.Reset()
	if box.value != 0 || len(box.items) != 0 || cap(box.items) != 3 {
		t.Fatalf("generic struct was not reset: %#v", box)
	}
}
`)

	if err := generate(dir); err != nil {
		t.Fatalf("generate reset methods: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated package tests failed: %v\n%s", err, output)
	}
}

func TestGenerateRemovesStaleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sample.go"), `package sample

type Target struct {
	value int
}
`)
	writeFile(t, filepath.Join(dir, generatedFileName), generatedFileHeader+`
package sample
`)

	if err := generate(dir); err != nil {
		t.Fatalf("generate reset methods: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, generatedFileName)); !os.IsNotExist(err) {
		t.Fatalf("generated file should be removed, got err %v", err)
	}
}

func TestGenerateKeepsForeignResetFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sample.go"), `package sample

type Target struct {
	value int
}
`)
	writeFile(t, filepath.Join(dir, generatedFileName), `package sample
`)

	if err := generate(dir); err != nil {
		t.Fatalf("generate reset methods: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, generatedFileName)); err != nil {
		t.Fatalf("foreign reset file should be kept: %v", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func assertContains(t *testing.T, source string, want string) {
	t.Helper()

	if !strings.Contains(source, want) {
		t.Fatalf("generated source does not contain %q:\n%s", want, source)
	}
}
