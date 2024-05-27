package middleware

import (
	"net/http"
)

func TextContentHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/plain" {
			http.Error(w, "Content-Type is not text/plain", http.StatusUnsupportedMediaType)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func JSONContentHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type is not application/json", http.StatusUnsupportedMediaType)
			return
		}
		h.ServeHTTP(w, r)
	})
}
