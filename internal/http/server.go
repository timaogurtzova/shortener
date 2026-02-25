package http

import (
	"log"
	"net/http"
)

type Server struct {
	mux  *http.ServeMux
	addr string
}

func NewServer(addr string, createHandler, redirectHandler http.Handler) *Server {
	mux := http.NewServeMux()

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			createHandler.ServeHTTP(w, r)
		} else {
			redirectHandler.ServeHTTP(w, r)
		}
	}))

	return &Server{
		mux:  mux,
		addr: addr,
	}
}

// Run запускает HTTP-сервер на указанном адресе
func (s *Server) Run() {
	log.Printf("Server started at %s\n", s.addr)
	if err := http.ListenAndServe(s.addr, s.mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
