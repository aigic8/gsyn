package api

import (
	"net/http"

	"github.com/aigic8/gosyn/api/handlers"
	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

func Router(spaces map[string]string, users map[string]utils.UserInfo) *chi.Mux {
	r := chi.NewRouter()

	// r.Use(middleware.AllowContentType("application/json"))
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Recoverer)

	r.Use(utils.UserAuthMiddleware(users))

	dirHandler := handlers.DirHandler{Spaces: spaces}
	r.Route("/api/dirs", func(r chi.Router) {
		r.Get("/list", dirHandler.GetList)
		r.Get("/tree", dirHandler.GetTree)
	})

	fileHandler := handlers.FileHandler{Spaces: spaces}
	r.Route("/api/files", func(r chi.Router) {
		r.Get("/", fileHandler.Get)
		r.Put("/new", fileHandler.PutNew)
		r.Get("/matches", fileHandler.Match)
		r.Get("/stat", fileHandler.Stat)
	})

	spaceHandler := handlers.SpaceHandler{}
	r.Route("/api/spaces", func(r chi.Router) {
		r.Get("/all", spaceHandler.GetAll)
	})

	return r
}

func Serve(r http.Handler, addr, certFile, privKeyFile string) error {
	server := http3.Server{
		Handler:    r,
		Addr:       addr,
		QuicConfig: &quic.Config{},
	}

	return server.ListenAndServeTLS(certFile, privKeyFile)
}
