package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"game-api/game"
	"game-api/utils"
)

// POST /games - Create a new game
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate game ID and create game
	gameID := utils.GenerateID(8)
	g := game.NewGame(gameID)
	s.addGame(g)

	// Return game info
	response := map[string]interface{}{
		"game_id":   g.ID,
		"locations": g.Locations,
		"message":   "Game created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Router for /games/{gameID}/...
func (s *Server) handleGameRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse path: /games/{gameID}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/games/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	gameID := parts[0]

	// Check if game exists
	g := s.getGame(gameID)
	if g == nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if len(parts) == 1 {
		s.handleGetGame(w, r, g)
	} else {
		switch parts[1] {
		case "players":
			s.handlePlayers(w, r, g)
		case "events":
			s.handleSSE(w, r, g)
		case "actions":
			s.handleActions(w, r, g)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

// GET /games/{gameID} - Get game state
func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	g.Mu.RLock()
	defer g.Mu.RUnlock()

	response := map[string]interface{}{
		"game_id":   g.ID,
		"locations": g.Locations,
		"players":   g.Players,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// POST /games/{gameID}/players - Add a player
func (s *Server) handlePlayers(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Create player at a random starting location
	playerID := utils.GenerateID(6)
	startLocation := g.GetRandomLocation()
	if startLocation == nil {
		http.Error(w, "No locations available", http.StatusInternalServerError)
		return
	}

	player := &game.Player{
		ID:              playerID,
		Name:            req.Name,
		CurrentLocation: startLocation.ID,
		Health:          100,
	}

	g.AddPlayer(player)

	token, err := s.generateToken(g.ID, playerID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"player":  player,
		"token":   token,
		"message": "Player created successfully.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
		return
	}

	claims, err := s.validateToken(parts[1])
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	if claims.GameID != g.ID {
		http.Error(w, "Token not valid for this game", http.StatusForbidden)
		return
	}

	playerID := claims.PlayerID

	var req struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	player := g.GetPlayer(playerID)
	if player == nil {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	switch req.Action {
	case "move":
		if err := g.MovePlayer(playerID, req.Target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Player moved to " + req.Target,
		})

	case "attack":
		if err := g.AttackPlayer(playerID, req.Target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Attack executed",
		})

	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// GET /games/{gameID}/events - SSE endpoint
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require JWT authentication
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
		return
	}

	claims, err := s.validateToken(parts[1])
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	if claims.GameID != g.ID {
		http.Error(w, "Token not valid for this game", http.StatusForbidden)
		return
	}

	playerID := claims.PlayerID

	// Verify player exists
	player := g.GetPlayer(playerID)
	if player == nil {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create event channel for this client
	eventChan := make(chan game.Event, 10)
	g.AddClient(eventChan, playerID) // Pass playerID
	defer g.RemoveClient(eventChan)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send welcome message with current location
	welcomeEvent := game.Event{
		Type:      "connected",
		Message:   fmt.Sprintf("Connected to game. You are in %s", player.CurrentLocation),
		Location:  player.CurrentLocation,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(welcomeEvent)
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	flusher.Flush()

	// Keep connection alive with periodic pings
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventChan:
			// Events are already filtered by BroadcastEvent
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			w.Write([]byte("data: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()

		case <-ticker.C:
			// Send keepalive comment
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
