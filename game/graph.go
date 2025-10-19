package game

import (
	"fmt"
	"game-api/utils"
	"math/rand"
	"time"
)

type Location struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Connections []string `json:"connections"` // IDs of connected locations
}

func GenerateGraph(numLocations int) map[string]*Location {
	locations := make(map[string]*Location)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	locationNames := []string{
		"Dark Forest", "Ancient Castle", "Misty Mountains", "Crystal Cave",
		"Desert Ruins", "Frozen Lake", "Abandoned Mine", "Haunted Graveyard",
		"Dragon's Lair", "Enchanted Garden", "Pirate Cove", "Volcano Peak",
	}

	// Create locations
	for i := 0; i < numLocations; i++ {
		id := utils.GenerateID(8)
		name := locationNames[i%len(locationNames)]
		if i >= len(locationNames) {
			name = fmt.Sprintf("%s %d", name, i/len(locationNames))
		}

		locations[id] = &Location{
			ID:          id,
			Name:        name,
			Description: fmt.Sprintf("A mysterious %s", name),
			Connections: []string{},
		}
	}

	// Convert map to slice for easier indexing
	locSlice := make([]*Location, 0, len(locations))
	for _, loc := range locations {
		locSlice = append(locSlice, loc)
	}

	// Connect locations randomly (ensure at least one connection per location)
	for i, loc := range locSlice {
		numConnections := rng.Intn(3) + 1 // 1-3 connections

		for j := 0; j < numConnections; j++ {
			targetIdx := rng.Intn(len(locSlice))
			if targetIdx != i { // Don't connect to self
				target := locSlice[targetIdx]

				// Add bidirectional connection if not already connected
				if !contains(loc.Connections, target.ID) {
					loc.Connections = append(loc.Connections, target.ID)
				}
				if !contains(target.Connections, loc.ID) {
					target.Connections = append(target.Connections, loc.ID)
				}
			}
		}

		// Ensure at least one connection
		if len(loc.Connections) == 0 && len(locSlice) > 1 {
			targetIdx := (i + 1) % len(locSlice)
			target := locSlice[targetIdx]
			loc.Connections = append(loc.Connections, target.ID)
			target.Connections = append(target.Connections, loc.ID)
		}
	}

	return locations
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
