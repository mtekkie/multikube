package multikube

import (
	"context"
	"log"
	"net/http"
)

type MiddlewareFunc func(next http.HandlerFunc) http.HandlerFunc

func WithEmpty(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}

func WithContext(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "Username", "amimof")
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func WithLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery, r.RemoteAddr, r.Proto)
		next.ServeHTTP(w, r)
	}
}
