package game

import (
	"fmt"
	"sync"
	"time"
)

type Game struct {
	ID        string
	Locations map[string]*Location
	Players   map[string]*Player

	clientPlayers map[chan Event]string

	Mu        sync.RWMutex
	ClientsMu sync.Mutex
}

func NewGame(id string) *Game {
	return &Game{
		ID:            id,
		Locations:     GenerateGraph(10),
		Players:       make(map[string]*Player),
		clientPlayers: make(map[chan Event]string),
		Mu:            sync.RWMutex{},
		ClientsMu:     sync.Mutex{},
	}
}

// Update AddClient to track player ID
func (g *Game) AddClient(ch chan Event, playerID string) {
	g.ClientsMu.Lock()
	defer g.ClientsMu.Unlock()
	g.clientPlayers[ch] = playerID
}

func (g *Game) RemoveClient(ch chan Event) {
	g.ClientsMu.Lock()
	defer g.ClientsMu.Unlock()
	delete(g.clientPlayers, ch)
	close(ch)
}

// Smart broadcast - only send events to players who can see them
func (g *Game) BroadcastEvent(event Event) {
	g.ClientsMu.Lock()
	defer g.ClientsMu.Unlock()

	event.Timestamp = time.Now()

	for clientChan, playerID := range g.clientPlayers {
		// Check if this player should see this event
		if g.shouldPlayerSeeEvent(playerID, event) {
			select {
			case clientChan <- event:
			default:
				// Client not responsive, remove it
				close(clientChan)
				delete(g.clientPlayers, clientChan)
			}
		}
	}
}

// Determine if a player should see an event
func (g *Game) shouldPlayerSeeEvent(playerID string, event Event) bool {
	// Global events are visible to everyone
	if event.Global {
		return true
	}

	// Get player's current location
	g.Mu.RLock()
	player := g.Players[playerID]
	g.Mu.RUnlock()

	if player == nil {
		return false
	}

	// Player sees events in their current location
	return event.Location == player.CurrentLocation
}

func (g *Game) GetPlayer(id string) *Player {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return g.Players[id]
}

func (g *Game) GetRandomLocation() *Location {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	for _, loc := range g.Locations {
		return loc
	}
	return nil
}

// game/game.go - Update these methods

func (g *Game) AddPlayer(player *Player) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	g.Players[player.ID] = player

	// Player joined is a GLOBAL event - everyone sees it
	g.BroadcastEvent(Event{
		Type:     EventPlayerJoined,
		PlayerID: player.ID,
		Message:  player.Name + " joined the game",
		Global:   true, // ← Visible to all
	})
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

	// Broadcast departure to OLD location
	g.BroadcastEvent(Event{
		Type:     EventPlayerMoved,
		PlayerID: playerID,
		Location: oldLocation, // ← People in old location see this
		Message:  fmt.Sprintf("%s left the area", player.Name),
	})

	// Broadcast arrival to NEW location
	g.BroadcastEvent(Event{
		Type:     EventPlayerMoved,
		PlayerID: playerID,
		Location: locationID, // ← People in new location see this
		Message:  fmt.Sprintf("%s arrived", player.Name),
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

	damage := 10
	target.Health -= damage

	// Attack event only visible in current location
	g.BroadcastEvent(Event{
		Type:     EventPlayerAttack,
		PlayerID: attackerID,
		TargetID: targetID,
		Location: attacker.CurrentLocation, // ← Only players here see this
		Message:  fmt.Sprintf("%s attacked %s for %d damage", attacker.Name, target.Name, damage),
	})

	if target.Health <= 0 {
		target.Health = 0
		g.BroadcastEvent(Event{
			Type:     EventPlayerLeft,
			PlayerID: targetID,
			Location: attacker.CurrentLocation,
			Message:  fmt.Sprintf("%s has been defeated!", target.Name),
		})
	}

	return nil
}
