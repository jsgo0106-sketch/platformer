package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	playersMu.Lock()
	var id int
	if len(freeIds) > 0 {
		id = freeIds[0]
		freeIds = freeIds[1:]
	} else {
		id = nextId
		nextId++
	}
	color := colors[(id-1)%len(colors)]
	players[id] = &Player{Id: id, X: 100, Y: 100, Color: color, Facing: 1}
	inputs[id] = &InputState{}
	playersMu.Unlock()

	connMu.Lock()
	connections[id] = conn
	connMu.Unlock()

	log.Printf("Player %d connected (%s). Total: %d\n", id, color, len(connections))

	playersMu.Lock()
	// Create a safe copy of players for the init message
	playersCopy := make(map[int]Player)
	for pid, p := range players {
		playersCopy[pid] = *p
	}
	initMsg := map[string]interface{}{
		"type":         "init",
		"playerId":     id,
		"players":      playersCopy,
		"platforms":    platforms,
		"bots":         bots,
		"orbs":         orbs,
		"thrownOrbs":   thrownOrbs,
		"explosions":   explosions,
		"chatMessages": chatMessages,
		"worldWidth":   WorldWidth,
		"worldHeight":  WorldHeight,
	}
	playersMu.Unlock()
	data, _ := json.Marshal(initMsg)
	conn.WriteMessage(websocket.TextMessage, data)

	playersMu.Lock()
	broadcast(map[string]interface{}{
		"type":     "playerJoined",
		"playerId": id,
		"player":   players[id],
	}, id)

	chatMessages = append(chatMessages, ChatMessage{
		PlayerId: 0, Color: "#888888",
		Text: fmt.Sprintf("Player %d (%s) joined", id, color),
		Time: float64(time.Now().UnixNano()) / 1e9,
	})
	playersMu.Unlock()

	defer func() {
		connMu.Lock()
		delete(connections, id)
		connMu.Unlock()

		playersMu.Lock()
		delete(players, id)
		delete(inputs, id)
		freeIds = append(freeIds, id)
		chatMessages = append(chatMessages, ChatMessage{
			PlayerId: 0, Color: "#888888",
			Text: fmt.Sprintf("Player %d left", id),
			Time: float64(time.Now().UnixNano()) / 1e9,
		})
		playersMu.Unlock()

		broadcast(map[string]interface{}{
			"type":     "playerLeft",
			"playerId": id,
		}, id)

		log.Printf("Player %d disconnected. Total: %d\n", id, len(connections))
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var input InputState
		if err := json.Unmarshal(msg, &input); err != nil {
			continue
		}

		playersMu.Lock()
		if input.ChatMsg != "" && len(input.ChatMsg) <= 200 {
			chatMessages = append(chatMessages, ChatMessage{
				PlayerId: id, Color: players[id].Color,
				Text: input.ChatMsg,
				Time: float64(time.Now().UnixNano()) / 1e9,
			})
			input.ChatMsg = ""
		}
		inputs[id] = &input
		playersMu.Unlock()
	}
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)

	// Spawn bots
	botCount := 4
	botColors := []string{"#ff4444", "#ff8844", "#cc44cc", "#44cc44", "#4488ff", "#cccc44"}
	for i := 0; i < botCount; i++ {
		id := -(i + 1)
		spawnX := float64(300 + i*300)
		color := botColors[i%len(botColors)]
		facing := 1.0
		if i%2 == 1 {
			facing = -1.0
		}
		bots = append(bots, Bot{
			Id: id,
			Player: Player{
				Id:     id,
				X:      spawnX,
				Y:      100,
				Color:  color,
				Facing: facing,
				IsBot:  true,
			},
		})
	}

	buildWaypointGraph()
	for i, wp := range waypoints {
    log.Printf("Waypoint %d: X=%.0f Y=%.0f", i, wp.X, wp.Y)
	}
	go gameLoop()

	fmt.Println("Server running on http://0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}