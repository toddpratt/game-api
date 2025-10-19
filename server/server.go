package server

import (
	"crypto/rand"
	"net/http"
	"sync"

	"game-api/game"
)

type Server struct {
	games   map[string]*game.Game
	gamesMu sync.RWMutex

	router    *http.ServeMux
	jwtSecret []byte
}

func NewServer() *Server {
	s := &Server{
		games:     make(map[string]*game.Game),
		router:    http.NewServeMux(),
		jwtSecret: generateSecret(),
	}

	s.registerRoutes()
	return s
}

func generateSecret() []byte {
	secret := make([]byte, 32)
	rand.Read(secret)
	return secret
}
func (s *Server) registerRoutes() {
	s.router.HandleFunc("/games", s.handleCreateGame)
	s.router.HandleFunc("/games/", s.handleGameRoutes)
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
