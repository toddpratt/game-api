package game

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Game struct {
	ID        string
	Locations map[string]*Location
	Players   map[string]*Player

	// SSE clients
	clients map[chan Event]bool

	Mu        sync.RWMutex
	clientsMu sync.Mutex
}

func NewGame(id string) *Game {
	return &Game{
		ID:        id,
		Locations: GenerateGraph(10),
		Players:   make(map[string]*Player),
		clients:   make(map[chan Event]bool),
	}
}

func (g *Game) AddPlayer(player *Player) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	g.Players[player.ID] = player
	g.BroadcastEvent(Event{
		Type:     EventPlayerJoined,
		PlayerID: player.ID,
		Message:  player.Name + " joined the game",
	})
}

func (g *Game) BroadcastEvent(event Event) {
	g.clientsMu.Lock()
	defer g.clientsMu.Unlock()

	event.Timestamp = time.Now()
	for clientChan := range g.clients {
		select {
		case clientChan <- event:
		default:
			close(clientChan)
			delete(g.clients, clientChan)
		}
	}
}

func (g *Game) GetPlayer(id string) *Player {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return g.Players[id]
}

func (g *Game) GetRandomLocation() *Location {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	if len(g.Locations) == 0 {
		return nil // Safety check
	}

	// Random selection
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := rng.Intn(len(g.Locations))

	i := 0
	for _, loc := range g.Locations {
		if i == idx {
			return loc
		}
		i++
	}

	return nil
}

func (g *Game) MovePlayer(playerID, locationID string) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	player := g.Players[playerID]
	if player == nil {
		return fmt.Errorf("player not found")
	}

	location := g.Locations[locationID]
	if location == nil {
		return fmt.Errorf("location not found")
	}

	// Check if move is valid (location is connected)
	currentLoc := g.Locations[player.CurrentLocation]
	connected := false
	for _, connID := range currentLoc.Connections {
		if connID == locationID {
			connected = true
			break
		}
	}

	if !connected && player.CurrentLocation != locationID {
		return fmt.Errorf("location not connected")
	}

	oldLocation := player.CurrentLocation
	player.CurrentLocation = locationID

	g.BroadcastEvent(Event{
		Type:     EventPlayerMoved,
		PlayerID: playerID,
		Location: locationID,
		Message:  fmt.Sprintf("%s moved from %s to %s", player.Name, oldLocation, locationID),
	})

	return nil
}

func (g *Game) AttackPlayer(attackerID, targetID string) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	attacker := g.Players[attackerID]
	target := g.Players[targetID]

	if attacker == nil || target == nil {
		return fmt.Errorf("player not found")
	}

	if attacker.CurrentLocation != target.CurrentLocation {
		return fmt.Errorf("players not in same location")
	}

	damage := 10 // Simple damage system
	target.Health -= damage

	g.BroadcastEvent(Event{
		Type:     EventPlayerAttack,
		PlayerID: attackerID,
		TargetID: targetID,
		Location: attacker.CurrentLocation,
		Message:  fmt.Sprintf("%s attacked %s for %d damage", attacker.Name, target.Name, damage),
	})

	if target.Health <= 0 {
		target.Health = 0
		g.BroadcastEvent(Event{
			Type:     EventPlayerLeft,
			PlayerID: targetID,
			Message:  fmt.Sprintf("%s has been defeated!", target.Name),
		})
	}

	return nil
}

func (g *Game) AddClient(ch chan Event) {
	g.clientsMu.Lock()
	defer g.clientsMu.Unlock()
	g.clients[ch] = true
}

func (g *Game) RemoveClient(ch chan Event) {
	g.clientsMu.Lock()
	defer g.clientsMu.Unlock()
	delete(g.clients, ch)
	close(ch)
}
