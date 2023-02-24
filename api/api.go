package main

import (
	"github.com/aigic8/gosyn/api/handlers"
	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Router(spaces map[string]string, users map[string]utils.UserInfo) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.AllowContentType("application/json"))
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Recoverer)

	r.Use(utils.UserAuthMiddleware(users))

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

	spaceHandler := handlers.SpaceHandler{}
	r.Route("/api/spaces", func(r chi.Router) {
		r.Get("/all", spaceHandler.GetAll)
	})

	return r
}
