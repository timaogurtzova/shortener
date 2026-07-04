package audit

import (
	"bytes"

	resetpool "github.com/timaogurtzova/shortener/pkg/pool"
)

var jsonBufferPool = resetpool.New(func() *bytes.Buffer {
	return new(bytes.Buffer)
})
