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

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createGame(w, r)
	case http.MethodGet:
		s.listGames(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	gameID := utils.GenerateID(8)
	g := game.NewGame(gameID)
	s.addGame(g)

	response := map[string]interface{}{
		"game_id":   g.ID,
		"locations": g.Locations,
		"message":   "Game created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	s.gamesMu.RLock()
	defer s.gamesMu.RUnlock()

	type GameSummary struct {
		ID            string `json:"id"`
		PlayerCount   int    `json:"player_count"`
		LocationCount int    `json:"location_count"`
	}

	games := make([]GameSummary, 0, len(s.games))

	for _, g := range s.games {
		g.Mu.RLock()
		summary := GameSummary{
			ID:            g.ID,
			PlayerCount:   len(g.Players),
			LocationCount: len(g.Locations),
		}
		g.Mu.RUnlock()

		games = append(games, summary)
	}

	response := map[string]interface{}{
		"games": games,
		"count": len(games),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGameRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/games/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	gameID := parts[0]
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
			if len(parts) == 3 && parts[2] == "me" {
				s.handleGetPlayerContext(w, r, g) // ← New endpoint
			} else {
				s.handlePlayers(w, r, g)
			}
		case "events":
			s.handleSSE(w, r, g)
		case "actions":
			s.handleActions(w, r, g)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

func (s *Server) handleGetPlayerContext(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodGet {
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

	g.Mu.RLock()
	defer g.Mu.RUnlock()

	player := g.Players[playerID]
	if player == nil {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	currentLocation := g.Locations[player.CurrentLocation]
	if currentLocation == nil {
		http.Error(w, "Current location not found", http.StatusInternalServerError)
		return
	}

	connectedLocations := make([]*game.Location, 0, len(currentLocation.Connections))
	for _, connID := range currentLocation.Connections {
		if loc := g.Locations[connID]; loc != nil {
			connectedLocations = append(connectedLocations, loc)
		}
	}

	playersHere := make([]*game.Player, 0)
	for _, p := range g.Players {
		if p.CurrentLocation == player.CurrentLocation && p.ID != playerID {
			playersHere = append(playersHere, p)
		}
	}

	response := map[string]interface{}{
		"player":              player,
		"current_location":    currentLocation,
		"connected_locations": connectedLocations,
		"players_here":        playersHere,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

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

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request, g *game.Game) {
	if r.Method != http.MethodGet {
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

	player := g.GetPlayer(playerID)
	if player == nil {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	eventChan := make(chan game.Event, 10)
	g.AddClient(eventChan, playerID) // Pass playerID
	defer g.RemoveClient(eventChan)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

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

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventChan:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			w.Write([]byte("data: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()

		case <-ticker.C:
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
