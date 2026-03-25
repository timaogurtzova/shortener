package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		log.Info().
			Str("uri", r.RequestURI).
			Str("method", r.Method).
			Str("duration", time.Since(start).String()).
			Int("status", status).
			Int("size", ww.BytesWritten()).
			Msg("HTTP request completed")
	})
}
