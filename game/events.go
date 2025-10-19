package game

import "time"

type EventType string

const (
	EventPlayerJoined EventType = "player_joined"
	EventPlayerLeft   EventType = "player_left"
	EventPlayerMoved  EventType = "player_moved"
	EventPlayerAttack EventType = "player_attack"
	EventNPCAction    EventType = "npc_action"
)

type Event struct {
	Type      EventType `json:"type"`
	PlayerID  string    `json:"player_id,omitempty"`
	Location  string    `json:"location,omitempty"`
	TargetID  string    `json:"target_id,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
