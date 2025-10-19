package game

type Player struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CurrentLocation string `json:"current_location"`
	Health          int    `json:"health"`
}
