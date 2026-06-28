package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
)

func spawnOrb() {
	// Pick a random platform to spawn on
	plat := platforms[rand.Intn(len(platforms))]
	x := plat.X + float64(rand.Intn(int(plat.Width)-20)) + 10
	y := plat.Y - OrbSize - 5 // Just above the platform
	orbType := "down"
	if rand.Intn(3) == 0 { // 1 in 3 chance of up orb
		orbType = "up"
	}
	orbs = append(orbs, Orb{X: x, Y: y, Timer: 15, OrbType: orbType})
	log.Printf("Orb spawned at X=%.0f Y=%.0f type=%s\n", x, y, orbType)
}

func broadcast(msg map[string]interface{}, excludeId int) {
	data, _ := json.Marshal(msg)
	connMu.Lock()
	defer connMu.Unlock()
	for id, conn := range connections {
		if id != excludeId {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func broadcastAll(msg map[string]interface{}) {
	broadcast(msg, -1)
}

func gameLoop() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	lastOrbSpawn = float64(time.Now().UnixNano()) / 1e9

	for range ticker.C {
		playersMu.Lock()
		// Safety limits
		if len(orbs) > 20 {
			orbs = orbs[len(orbs)-10:] // Keep last 10
		}
		if len(explosions) > 30 {
			explosions = nil // Clear old explosions
		}
		if len(thrownOrbs) > 20 {
			thrownOrbs = thrownOrbs[len(thrownOrbs)-10:]
		}
		now := float64(time.Now().UnixNano()) / 1e9

		// Spawn orb
		if now-lastOrbSpawn >= OrbSpawnRate {
			spawnOrb()
			lastOrbSpawn = now
		}

		// Update world orb timers
		var remainingOrbs []Orb
		for _, o := range orbs {
			o.Timer -= 1.0 / 60.0
			if o.Timer > 0 {
				remainingOrbs = append(remainingOrbs, o)
			}
		}
		orbs = remainingOrbs

		// Clean old chat messages
		if len(chatMessages) > 50 {
			chatMessages = chatMessages[len(chatMessages)-50:]
		}

		// Handle orb pickup
		for oi, o := range orbs {
			pickedUp := false
			for pid, p := range players {
				if p.HasOrb || p.RespawnTimer > 0 {
					continue
				}
				hasThrown := false
				for _, to := range thrownOrbs {
					if to.OwnerId == pid {
						hasThrown = true
						break
					}
				}
				if hasThrown {
					continue
				}
				dist := math.Sqrt((p.X+PlayerSize/2-o.X-OrbSize/2)*(p.X+PlayerSize/2-o.X-OrbSize/2) +
					(p.Y+PlayerSize/2-o.Y-OrbSize/2)*(p.Y+PlayerSize/2-o.Y-OrbSize/2))
				if dist < PickupRange {
					p.HasOrb = true
					p.HeldOrbType = o.OrbType
					log.Printf("Player %d picked up %s orb\n", pid, o.OrbType)
					pickedUp = true
					break
				}
			}
			if pickedUp {
				orbs = append(orbs[:oi], orbs[oi+1:]...)
				break
			}
		}
		
		// Bot orb pickup
		for oi, o := range orbs {
			pickedUp := false
			for i := range bots {
				if bots[i].HasOrb || bots[i].RespawnTimer > 0 || bots[i].OrbThrown {
					continue
				}
				dist := math.Sqrt((bots[i].X+PlayerSize/2-o.X-OrbSize/2)*(bots[i].X+PlayerSize/2-o.X-OrbSize/2) +
					(bots[i].Y+PlayerSize/2-o.Y-OrbSize/2)*(bots[i].Y+PlayerSize/2-o.Y-OrbSize/2))
				if dist < PickupRange {
					bots[i].HasOrb = true
					bots[i].HeldOrbType = o.OrbType
					bots[i].OrbPickupTime = now
					log.Printf("Bot %d picked up %s orb\n", bots[i].Id, o.OrbType)
					pickedUp = true
					break
				}
			}
			if pickedUp {
				orbs = append(orbs[:oi], orbs[oi+1:]...)
				break
			}
		}

		// Handle throw input
		for pid, p := range players {
			inp := inputs[pid]
			if inp == nil {
				continue
			}
			if p.HasOrb && inp.ThrowOrb {
				var vx, vy float64
				if p.HeldOrbType == "up" {
					vx = p.Facing * ThrowSpeedXUp
					vy = ThrowSpeedYUp
				} else {
					vx = p.Facing * ThrowSpeedX
					vy = ThrowSpeedY
				}
				orb := ThrownOrb{
					X:       p.X + PlayerSize/2 - OrbSize/2,
					Y:       p.Y,
					Vx:      vx,
					Vy:      vy,
					FuseEnd: now + OrbFuseTime,
					OwnerId: pid,
					OrbType: p.HeldOrbType,
				}
				thrownOrbs = append(thrownOrbs, orb)
				p.HasOrb = false
				p.HeldOrbType = ""
			}
		}

		// Bot orb throws
		for i := range bots {
			inp := inputs[bots[i].Id]
			if inp == nil || !inp.ThrowOrb {
				continue
			}
			if bots[i].HasOrb {
				var vx, vy float64
				if bots[i].HeldOrbType == "up" {
					vx = bots[i].Facing * ThrowSpeedXUp
					vy = ThrowSpeedYUp
				} else {
					vx = bots[i].Facing * ThrowSpeedX
					vy = ThrowSpeedY
				}
				orb := ThrownOrb{
					X:       bots[i].X + PlayerSize/2 - OrbSize/2,
					Y:       bots[i].Y,
					Vx:      vx,
					Vy:      vy,
					FuseEnd: now + OrbFuseTime,
					OwnerId: bots[i].Id,
					OrbType: bots[i].HeldOrbType,
				}
				thrownOrbs = append(thrownOrbs, orb)
				bots[i].HasOrb = false
				bots[i].HeldOrbType = ""
				bots[i].OrbThrown = true
				bots[i].OrbThrowTime = now
			}
		}



		// Handle detonate input (owner only)
		for pid, inp := range inputs {
			if inp == nil || !inp.Detonate {
				continue
			}
			for i := range thrownOrbs {
				if thrownOrbs[i].OwnerId == pid && thrownOrbs[i].FuseEnd > now {
					thrownOrbs[i].FuseEnd = now
				}
			}
		}

		// Update thrown orbs
		for i := range thrownOrbs {
			applyOrbPhysics(&thrownOrbs[i])
		}

		// Check for explosions
		var remainingThrown []ThrownOrb
		for _, to := range thrownOrbs {
			if now >= to.FuseEnd {
				isHorizontal := to.OrbType == "up"
				explosions = append(explosions, Explosion{
					X: to.X + OrbSize/2, Y: to.Y + OrbSize/2,
					Timer: 0.6, MaxTimer: 0.6,
					IsHorizontal: isHorizontal,
				})

				if isHorizontal {
					// Horizontal lightning: kill players on same Y level
					for otherId, other := range players {
						if otherId == to.OwnerId || other.RespawnTimer > 0 {
							continue
						}
						if math.Abs(other.Y+PlayerSize/2-(to.Y+OrbSize/2)) < ExplosionWidth {
							other.X = float64(rand.Intn(400)) + 50
							other.Y = 100
							other.Vx = 0
							other.Vy = 0
							other.RespawnTimer = 1.5
							other.HasOrb = false
						}
					}
					// Kill bots in horizontal row (same Y)
					for i := range bots {
						if bots[i].RespawnTimer > 0 {
							continue
						}
						if math.Abs(bots[i].Y+PlayerSize/2-(to.Y+OrbSize/2)) < ExplosionWidth {
							bots[i].X = float64(rand.Intn(400)) + 50
							bots[i].Y = 100
							bots[i].Vx = 0
							bots[i].Vy = 0
							bots[i].RespawnTimer = 1.5
							bots[i].HasOrb = false
							bots[i].OrbThrown = false
						}
					}
				} else {
					// Vertical lightning (original): kill players on same X
					for otherId, other := range players {
						if otherId == to.OwnerId || other.RespawnTimer > 0 {
							continue
						}
						if math.Abs(other.X+PlayerSize/2-(to.X+OrbSize/2)) < ExplosionWidth {
							other.X = float64(rand.Intn(400)) + 50
							other.Y = 100
							other.Vx = 0
							other.Vy = 0
							other.RespawnTimer = 1.5
							other.HasOrb = false
						}
					}
					// Kill bots in vertical column (same X)
					for i := range bots {
						if bots[i].RespawnTimer > 0 {
							continue
						}
						if math.Abs(bots[i].X+PlayerSize/2-(to.X+OrbSize/2)) < ExplosionWidth {
							bots[i].X = float64(rand.Intn(400)) + 50
							bots[i].Y = 100
							bots[i].Vx = 0
							bots[i].Vy = 0
							bots[i].RespawnTimer = 1.5
							bots[i].HasOrb = false
							bots[i].OrbThrown = false
						}
					}
				}
			} else {
				remainingThrown = append(remainingThrown, to)
			}
		}
		thrownOrbs = remainingThrown
		
		// Reset bot orb state for any orb that just exploded
		for i := range bots {
			if bots[i].OrbThrown {
				stillActive := false
				for _, to := range thrownOrbs {
					if to.OwnerId == bots[i].Id {
						stillActive = true
						break
					}
				}
				if !stillActive {
					bots[i].OrbThrown = false
					bots[i].HeldOrbType = ""
				}
			}
		}

		// Update explosion timers
		var remainingExplosions []Explosion
		for _, ex := range explosions {
			ex.Timer -= 1.0 / 60.0
			if ex.Timer > 0 {
				remainingExplosions = append(remainingExplosions, ex)
			}
		}
		explosions = remainingExplosions

		// Apply player physics
		for id, p := range players {
			inp := inputs[id]
			if inp == nil {
				inp = &InputState{}
			}
			applyPhysics(p, inp, players, id, now)
		}

		// Update bots (always apply physics, only run AI when alive)
		for i := range bots {
			if bots[i].RespawnTimer <= 0 {
				updateBot(&bots[i], players, now)
			} else {
				applyPhysics(&bots[i].Player, &InputState{}, players, bots[i].Id, now)
			}
		}

		// Kill plane for players
		for _, p := range players {
			if p.Y > 610 && p.RespawnTimer <= 0 {
				p.X = float64(rand.Intn(400)) + 50
				p.Y = 100
				p.Vx = 0
				p.Vy = 0
				p.RespawnTimer = 1.5
				p.HasOrb = false
			}
		}

		// Kill plane for bots
		/*
		for i := range bots {
			if bots[i].Y > 590 && bots[i].RespawnTimer <= 0 {
				bots[i].X = float64(rand.Intn(400)) + 50
				bots[i].Y = 100
				bots[i].Vx = 0
				bots[i].Vy = 0
				bots[i].RespawnTimer = 1.5
			}
		}
		*/

		resolvePlayerOverlaps()
		resolveBotOverlaps()

		// Create a safe copy of players
		playersCopy := make(map[int]Player)
		for pid, p := range players {
			playersCopy[pid] = *p
		}

		state := map[string]interface{}{
			"type":         "gameState",
			"players":      playersCopy,
			"bots":         bots,
			"orbs":         orbs,
			"thrownOrbs":   thrownOrbs,
			"explosions":   explosions,
			"chatMessages": chatMessages,
		}

		playersMu.Unlock()
		broadcastAll(state)
	}
}