package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func CreateRoutes() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	var TradeHandler = NewTradeHandler()

	r.Route("/trade", func(r chi.Router) {
		r.Get("/", TradeHandler.Get)
		r.Delete("/", TradeHandler.DeleteAll)
	})

	return r
}
