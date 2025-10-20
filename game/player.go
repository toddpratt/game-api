package game

import "math/rand"

type Player struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CurrentLocation string `json:"current_location"`
	Health          int    `json:"health"`
	Strength        int    `json:"strength"`
	Dexterity       int    `json:"dexterity"`
}

// RollAttribute generates a random attribute value (3-18, simulating 3d6)
func RollAttribute() int {
	return rand.Intn(16) + 3 // Random number between 3 and 18 inclusive
}
