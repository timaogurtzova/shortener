package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

const gzipEncoding = "gzip"

// gzipBodyReadCloser закрывает и gzip.Reader, и исходное тело запроса.
type gzipBodyReadCloser struct {
	reader *gzip.Reader
	body   io.ReadCloser
}

func (g *gzipBodyReadCloser) Read(p []byte) (int, error) {
	return g.reader.Read(p)
}

func (g *gzipBodyReadCloser) Close() error {
	if err := g.reader.Close(); err != nil {
		_ = g.body.Close()
		return err
	}

	return g.body.Close()
}

func GunzipRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding, ok := requestContentEncoding(r.Header.Values("Content-Encoding"))
		if !ok {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}

		body, err := newGzipBody(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		r.Body = body
		r.Header.Del("Content-Encoding")
		r.Header.Del("Content-Length")
		r.ContentLength = -1

		next.ServeHTTP(w, r)
	})
}

func requestContentEncoding(values []string) (string, bool) {
	encodings := parseContentEncodings(values)
	if len(encodings) == 0 {
		return "", true
	}

	if len(encodings) == 1 && encodings[0] == gzipEncoding {
		return gzipEncoding, true
	}

	return "", false
}

func newGzipBody(body io.ReadCloser) (io.ReadCloser, error) {
	reader, err := gzip.NewReader(body)
	if err != nil {
		return nil, err
	}

	return &gzipBodyReadCloser{
		reader: reader,
		body:   body,
	}, nil
}

func parseContentEncodings(values []string) []string {
	var encodings []string

	for _, value := range values {
		for _, encoding := range strings.Split(value, ",") {
			encoding = strings.TrimSpace(strings.ToLower(encoding))
			if encoding == "" {
				continue
			}
			encodings = append(encodings, encoding)
		}
	}

	return encodings
}
