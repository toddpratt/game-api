# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a multiplayer game API built in Go that implements a location-based adventure game with real-time events via Server-Sent Events (SSE). Players can join games, move between connected locations, attack other players, and receive real-time notifications about events happening in their current location.

## Commands

### Running the server
```bash
go run main.go
```

### Building
```bash
go build -o game-api
```

### Running tests
```bash
go test ./...
```

### Running tests for a specific package
```bash
go test ./game
go test ./server
go test ./utils
```

## Environment Variables

The following environment variables can be configured:

- `PORT`: Server port (default: `8080`)
- `JWT_SECRET`: Hex-encoded JWT signing secret (if not set, a temporary secret is generated - NOT suitable for production)
- `ALLOWED_ORIGINS`: CORS allowed origins (default: `*`)

## Architecture

### Package Structure

- **main.go**: Entry point that loads config and starts the HTTP server
- **config/**: Configuration loading from environment variables
- **server/**: HTTP server, routing, handlers, and authentication
- **game/**: Core game logic, state management, and event system
- **utils/**: Shared utilities (ID generation)

### Key Architectural Patterns

#### In-Memory Game State
Games are stored in memory in the `Server` struct with a mutex-protected map (`games map[string]*Game`). Each `Game` also has its own mutex (`Mu`) protecting player and location state. There is NO database - all state is ephemeral.

#### Location Graph System
Each game generates a random connected graph of locations at creation time (see `game/graph.go`). Locations are bidirectionally connected, and players can only move to directly connected locations. The graph generation ensures all locations have at least one connection.

#### JWT Authentication
Players receive a JWT token when they join a game (via `POST /games/{gameID}/players`). The token contains:
- `player_id`: The player's unique ID
- `game_id`: The game they belong to

Tokens are required for:
- Performing actions (`POST /games/{gameID}/actions`)
- Receiving events (`GET /games/{gameID}/events`)
- Getting player context (`GET /games/{gameID}/players/me`)

#### Server-Sent Events (SSE) for Real-Time Updates
The event system is location-aware:
- Events can be `Global` (all players see them) or location-specific
- Each SSE client is associated with a specific player ID
- `BroadcastEvent()` filters events: players only receive events that are either global OR occurring in their current location
- Event filtering happens by checking player locations upfront (avoiding repeated lock acquisition)

**Critical deadlock prevention pattern**: Game state mutations (in `MovePlayer`, `AttackPlayer`, `AddPlayer`) acquire `g.Mu`, prepare events, release the lock, THEN call `BroadcastEvent()`. This prevents deadlocks between `g.Mu` (game state) and `g.ClientsMu` (client management).

#### Concurrency Safety
- `Server.games`: Protected by `Server.gamesMu` (RWMutex)
- `Game.Players` and `Game.Locations`: Protected by `Game.Mu` (RWMutex)
- `Game.clientPlayers`: Protected by `Game.ClientsMu` (Mutex)
- Always release game state locks BEFORE broadcasting events to prevent deadlocks

### API Endpoints

#### Game Management
- `POST /games`: Create a new game, returns game ID and generated locations
- `GET /games`: List all active games with player/location counts
- `GET /games/{gameID}`: Get full game state (locations and players)

#### Player Management
- `POST /games/{gameID}/players`: Join a game with a name, receive JWT token and starting location
- `GET /games/{gameID}/players/me`: Get current player's context (requires JWT):
  - Current player info
  - Current location details
  - Connected locations (where player can move)
  - Other players in same location

#### Actions (all require JWT)
- `POST /games/{gameID}/actions`:
  - `{"action": "move", "target": "locationID"}`: Move to a connected location
  - `{"action": "attack", "target": "playerID"}`: Attack another player in same location

#### Real-Time Events
- `GET /games/{gameID}/events`: SSE endpoint (requires JWT)
  - Returns location-filtered events for the authenticated player
  - Includes periodic keepalive messages
  - Event types: `player_joined`, `player_left`, `player_moved`, `player_attack`

### Working with Events

When adding new event types:
1. Add the event type constant to `game/events.go`
2. Create the event with appropriate `Location` and `Global` fields
3. Use `Game.BroadcastEvent()` to send it (handles filtering automatically)
4. Ensure any game state modifications follow the lock-then-broadcast pattern

### Authentication Flow

1. Player joins game → receives JWT token with `player_id` and `game_id`
2. Player includes token in `Authorization: Bearer <token>` header
3. Server validates token and extracts claims
4. Server verifies `game_id` in token matches requested game
5. Player can perform actions and receive events for that game
