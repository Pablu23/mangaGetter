package server

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

func (s *Server) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("auth")
		log.Debug().Msg("Hit auth")
		if r.URL.Path == "/login" || r.URL.Path == "/login/" || r.URL.Path == "/healthz" {
			log.Debug().Msg("Hit /login")
			next.ServeHTTP(w, r)
			return
		}
		if s.secret == "" || (cookie != nil && cookie.Value == s.secret) {
			log.Debug().Str("path", r.URL.Path).Msg("Hit with correct secret")
			next.ServeHTTP(w, r)
		} else {
			log.Debug().Str("path", r.URL.Path).Msg("redirect to login")
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	})
}
