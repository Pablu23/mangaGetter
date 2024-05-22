package server

import (
	"net/http"
)

func (s *Server) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("auth")
		if r.URL.Path == "/login" || r.URL.Path == "/login/" {
			next.ServeHTTP(w, r)
			return
		}
		if cookie != nil && cookie.Value == s.secret {
			next.ServeHTTP(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	})
}
