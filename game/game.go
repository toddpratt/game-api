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

func (g *Game) shouldPlayerSeeEvent(playerID string, event Event) bool {
	if event.Global {
		return true
	}

	g.Mu.RLock()
	player := g.Players[playerID]
	g.Mu.RUnlock()

	if player == nil {
		return false
	}

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

func (g *Game) MovePlayer(playerID, locationID string) error {
	var departureEvent, arrivalEvent Event
	var oldLocation, newLocation string

	g.Mu.Lock()
	player := g.Players[playerID]
	if player == nil {
		g.Mu.Unlock()
		return fmt.Errorf("player not found")
	}

	location := g.Locations[locationID]
	if location == nil {
		g.Mu.Unlock()
		return fmt.Errorf("location not found")
	}

	currentLoc := g.Locations[player.CurrentLocation]
	connected := false
	for _, connID := range currentLoc.Connections {
		if connID == locationID {
			connected = true
			break
		}
	}

	if !connected && player.CurrentLocation != locationID {
		g.Mu.Unlock()
		return fmt.Errorf("location not connected")
	}

	oldLocation = player.CurrentLocation
	newLocation = locationID
	player.CurrentLocation = locationID

	departureEvent = Event{
		Type:     EventPlayerMoved,
		PlayerID: playerID,
		Location: oldLocation,
		Message:  fmt.Sprintf("%s left the area", player.Name),
	}

	arrivalEvent = Event{
		Type:     EventPlayerMoved,
		PlayerID: playerID,
		Location: newLocation,
		Message:  fmt.Sprintf("%s arrived", player.Name),
	}

	g.Mu.Unlock()

	g.BroadcastEvent(departureEvent)
	g.BroadcastEvent(arrivalEvent)

	return nil
}

func (g *Game) AttackPlayer(attackerID, targetID string) error {
	var attackEvent, defeatEvent Event
	var shouldBroadcastDefeat bool

	g.Mu.Lock()

	attacker := g.Players[attackerID]
	target := g.Players[targetID]

	if attacker == nil || target == nil {
		g.Mu.Unlock()
		return fmt.Errorf("player not found")
	}

	if attacker.CurrentLocation != target.CurrentLocation {
		g.Mu.Unlock()
		return fmt.Errorf("players not in same location")
	}

	damage := 10
	target.Health -= damage

	attackEvent = Event{
		Type:     EventPlayerAttack,
		PlayerID: attackerID,
		TargetID: targetID,
		Location: attacker.CurrentLocation,
		Message:  fmt.Sprintf("%s attacked %s for %d damage", attacker.Name, target.Name, damage),
	}

	if target.Health <= 0 {
		target.Health = 0
		shouldBroadcastDefeat = true
		defeatEvent = Event{
			Type:     EventPlayerLeft,
			PlayerID: targetID,
			Location: attacker.CurrentLocation,
			Message:  fmt.Sprintf("%s has been defeated!", target.Name),
		}
	}

	g.Mu.Unlock()

	g.BroadcastEvent(attackEvent)
	if shouldBroadcastDefeat {
		g.BroadcastEvent(defeatEvent)
	}

	return nil
}

func (g *Game) AddPlayer(player *Player) {
	g.Mu.Lock()
	g.Players[player.ID] = player
	g.Mu.Unlock()

	g.BroadcastEvent(Event{
		Type:     EventPlayerJoined,
		PlayerID: player.ID,
		Message:  player.Name + " joined the game",
		Global:   true,
	})
}

func (g *Game) BroadcastEvent(event Event) {
	event.Timestamp = time.Now()

	playerLocations := make(map[string]string)
	if !event.Global {
		g.Mu.RLock()
		for pid, player := range g.Players {
			playerLocations[pid] = player.CurrentLocation
		}
		g.Mu.RUnlock()
	}

	g.ClientsMu.Lock()
	defer g.ClientsMu.Unlock()

	for clientChan, playerID := range g.clientPlayers {
		shouldSee := event.Global || playerLocations[playerID] == event.Location

		if shouldSee {
			select {
			case clientChan <- event:
			default:
				close(clientChan)
				delete(g.clientPlayers, clientChan)
			}
		}
	}
}
