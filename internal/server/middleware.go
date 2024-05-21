package server

import (
	"net/http"
)

func (s *Server) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil {
			if r.URL.Path == "/login" || r.URL.Path == "/login/" {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if cookie.Value == s.secret {
			next.ServeHTTP(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	})
}
