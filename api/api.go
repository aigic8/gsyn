package main

import (
	"net/http"
	"strings"

	"github.com/aigic8/gosyn/api/handlers"
	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type UserInfo struct {
	GUID   string
	Spaces []string
}

func Router(spaces map[string]string, users map[string]UserInfo) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.AllowContentType("application/json"))
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Recoverer)

	// TODO test
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || headerParts[0] != "simple" {
				utils.WriteAPIErr(w, http.StatusUnauthorized, "bad authentication")
				return
			}

			if _, ok := users[headerParts[1]]; !ok {
				utils.WriteAPIErr(w, http.StatusUnauthorized, "bad authentication")
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	dirHandler := handlers.DirHandler{Spaces: spaces}
	r.Route("/api/dirs", func(r chi.Router) {
		r.Get("/list/{path}", dirHandler.GetList)
		r.Get("/tree/{path}", dirHandler.GetTree)
	})

	fileHandler := handlers.FileHandler{Spaces: spaces}
	r.Route("/api/files", func(r chi.Router) {
		r.Get("/{path}", fileHandler.Get)
		r.Put("/new", fileHandler.PutNew)
		r.Get("/matches/{path}", fileHandler.Match)
		r.Get("/stat/{path}", fileHandler.Stat)
	})

	spaceHandler := handlers.SpaceHandler{Spaces: spaces}
	r.Route("/api/spaces", func(r chi.Router) {
		r.Get("/all", spaceHandler.GetAll)
	})

	return r
}
