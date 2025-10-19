package server

import (
	"net/http"
	"sync"

	"game-api/config"
	"game-api/game"
)

type Server struct {
	games   map[string]*game.Game
	gamesMu sync.RWMutex

	router *http.ServeMux
	config *config.Config
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		games:  make(map[string]*game.Game),
		router: http.NewServeMux(),
		config: cfg,
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.router.HandleFunc("/games", s.corsMiddleware(s.handleCreateGame))
	s.router.HandleFunc("/games/", s.corsMiddleware(s.handleGameRoutes))
}

func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.config.AllowedOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Server) addGame(g *game.Game) {
	s.gamesMu.Lock()
	defer s.gamesMu.Unlock()
	s.games[g.ID] = g
}

func (s *Server) getGame(id string) *game.Game {
	s.gamesMu.RLock()
	defer s.gamesMu.RUnlock()
	return s.games[id]
}

func (s *Server) removeGame(id string) {
	s.gamesMu.Lock()
	defer s.gamesMu.Unlock()
	delete(s.games, id)
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
