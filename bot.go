package main

import (
	"math"
	"math/rand"
	"log"
)

func platformAhead(b *Bot, dir float64) bool {
	checkX := b.X + dir*PlayerSize
	checkY := b.Y + PlayerSize + 5
	for _, plat := range platforms {
		if checkX+PlayerSize > plat.X && checkX < plat.X+plat.Width {
			if checkY >= plat.Y && checkY <= plat.Y+plat.Height+20 {
				return true
			}
		}
	}
	return false
}

func gapAhead(b *Bot, dir float64) bool {
	checkX := b.X + dir*PlayerSize
	checkY := b.Y + PlayerSize + 5
	for _, plat := range platforms {
		if checkX+PlayerSize > plat.X && checkX < plat.X+plat.Width {
			if checkY >= plat.Y && checkY <= plat.Y+plat.Height+20 {
				return false
			}
		}
	}
	return b.Y+PlayerSize >= 540
}

func platformAbove(b *Bot) bool {
	checkY := b.Y - 20
	for _, plat := range platforms {
		if b.X+PlayerSize > plat.X && b.X < plat.X+plat.Width {
			if checkY >= plat.Y && checkY <= plat.Y+plat.Height {
				return true
			}
		}
	}
	return false
}

func shouldBotJump(b *Bot, targetY float64, now float64, chasingOrb bool) bool {
	if !b.OnGround {
		return false
	}

	cooldown := 1.0
	jumpThreshold := 40.0
	if chasingOrb {
		cooldown = 0.3
		jumpThreshold = 15.0
	}

	if targetY < b.Y-jumpThreshold && now-b.LastJump >= cooldown {
		return true
	}
	if now-b.LastJump >= 0.8 && platformAhead(b, b.Facing) && platformAbove(b) {
		return true
	}
	if now-b.LastJump >= 0.8 && gapAhead(b, b.Facing) {
		return true
	}
	return false
}

func shouldDropDown(b *Bot, targetY float64) bool {
	if b.Y < targetY-60 && b.OnGround {
		checkY := b.Y + PlayerSize + 20
		hasFloor := false
		for _, plat := range platforms {
			if b.X+PlayerSize > plat.X && b.X < plat.X+plat.Width {
				if checkY >= plat.Y && checkY <= plat.Y+plat.Height {
					hasFloor = true
					break
				}
			}
		}
		if !hasFloor {
			return false
		}
		return true
	}
	return false
}

func moveToward(b *Bot, targetX, targetY float64, now float64, chasingOrb bool) *InputState {
	input := &InputState{}
	botCX := b.X + PlayerSize/2

	if targetX < botCX-15 {
		input.Left = true
	} else if targetX > botCX+15 {
		input.Right = true
	}

	if shouldBotJump(b, targetY, now, chasingOrb) {
		input.Jump = true
		b.LastJump = now
	}

	return input
}

func updateBot(b *Bot, players map[int]*Player, now float64) {
	if b.RespawnTimer > 0 {
		return
	}

	// ─── Monitor thrown orb ───
	if b.OrbThrown {
		var myOrb *ThrownOrb
		for i := range thrownOrbs {
			if thrownOrbs[i].OwnerId == b.Id {
				myOrb = &thrownOrbs[i]
				break
			}
		}

		if myOrb != nil {
			detonateNow := false
			orbCX := myOrb.X + OrbSize/2
			orbCY := myOrb.Y + OrbSize/2

			for _, p := range players {
				if p.RespawnTimer > 0 {
					continue
				}
				if myOrb.OrbType == "down" {
					if math.Abs(p.X+PlayerSize/2-orbCX) < ExplosionWidth+20 {
						detonateNow = true
						break
					}
				} else {
					if math.Abs(p.Y+PlayerSize/2-orbCY) < ExplosionWidth+20 {
						detonateNow = true
						break
					}
				}
			}

			if detonateNow {
				if b.DetonateReadyTime == 0 {
					b.DetonateReadyTime = now + 0.15
				}
				if now >= b.DetonateReadyTime {
					myOrb.FuseEnd = now
					b.DetonateReadyTime = 0
				}
			} else {
				b.DetonateReadyTime = 0
			}

			if myOrb.FuseEnd-now < 0.1 && b.DetonateReadyTime == 0 {
				myOrb.FuseEnd = now
			}
		} else {
			b.OrbThrown = false
			b.HeldOrbType = ""
			b.DetonateReadyTime = 0
		}

		var nearestPlayer *Player
		minDist := 99999.0
		for _, p := range players {
			if p.RespawnTimer > 0 {
				continue
			}
			dist := math.Sqrt((p.X-b.X)*(p.X-b.X) + (p.Y-b.Y)*(p.Y-b.Y))
			if dist < minDist {
				minDist = dist
				nearestPlayer = p
			}
		}

		input := &InputState{}
		if rand.Intn(120) == 0 {
			input.Dash = true
			log.Printf("Bot %d DASH!", b.Id)
		}
		if nearestPlayer != nil && minDist < 600 {
			if nearestPlayer.X < b.X-30 {
				input.Left = true
			} else if nearestPlayer.X > b.X+30 {
				input.Right = true
			}
			if shouldBotJump(b, nearestPlayer.Y, now, false) {
				input.Jump = true
				b.LastJump = now
			}
		}
		inputs[b.Id] = input
		applyPhysics(&b.Player, input, players, b.Id, now)
		return
	}

	// ─── Holding an orb ───
	if b.HasOrb {
		var nearest *Player
		minDist := 99999.0
		for _, p := range players {
			if p.RespawnTimer > 0 {
				continue
			}
			dist := math.Sqrt((p.X-b.X)*(p.X-b.X) + (p.Y-b.Y)*(p.Y-b.Y))
			if dist < minDist {
				minDist = dist
				nearest = p
			}
		}

		if nearest != nil {
			if nearest.X < b.X {
				b.Facing = -1
			} else {
				b.Facing = 1
			}

			throwX := b.X + b.Facing*150
			playerDist := math.Abs(nearest.X - throwX)
			shouldThrow := false

			if b.HeldOrbType == "down" {
				if nearest.Y >= b.Y-60 && playerDist < 200 {
					shouldThrow = true
				}
			} else {
				if math.Abs(nearest.Y-b.Y) < 80 && playerDist < 250 {
					shouldThrow = true
				}
			}

			if now-b.OrbPickupTime >= 3.0 {
				shouldThrow = true
			}

			if shouldThrow && now-b.OrbPickupTime >= 0.4 && minDist < 400 {
				input := &InputState{}
				input.ThrowOrb = true
				b.OrbThrown = true
				b.OrbThrowTime = now
				inputs[b.Id] = input
				applyPhysics(&b.Player, input, players, b.Id, now)
				return
			}

			input := &InputState{}
			if rand.Intn(120) == 0 {
			input.Dash = true
			log.Printf("Bot %d DASH!", b.Id)
			}	
			backOffDist := 150.0
			currentDist := math.Abs(nearest.X - b.X)
			if currentDist < backOffDist {
				if nearest.X < b.X {
					input.Right = true
				} else {
					input.Left = true
				}
			} else if currentDist > backOffDist+30 && int(now*10)%3 != 0 {
				if nearest.X < b.X {
					input.Left = true
				} else {
					input.Right = true
				}
			}

			if shouldBotJump(b, nearest.Y, now, false) {
				input.Jump = true
				b.LastJump = now
			}
			inputs[b.Id] = input
			applyPhysics(&b.Player, input, players, b.Id, now)
			return
		}

		// No player exists - patrol
		input := &InputState{}
		if b.Facing > 0 {
			input.Right = true
		} else {
			input.Left = true
		}
		if b.X < 50 {
			b.Facing = 1
		} else if b.X > WorldWidth-80 {
			b.Facing = -1
		} else if now-b.ChangeTimer > 3.0 {
			b.Facing = -b.Facing
			b.ChangeTimer = now
		}
		if shouldBotJump(b, b.Y-50, now, false) {
			input.Jump = true
			b.LastJump = now
		}
		inputs[b.Id] = input
		applyPhysics(&b.Player, input, players, b.Id, now)
		return
	}

	// ─── No orb - find orb or chase player ───
	var nearestOrb *Orb
	minOrbDist := 99999.0
	for i := range orbs {
		dx := b.X + PlayerSize/2 - orbs[i].X - OrbSize/2
		dy := b.Y + PlayerSize/2 - orbs[i].Y - OrbSize/2
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist < minOrbDist {
			minOrbDist = dist
			nearestOrb = &orbs[i]
		}
	}

	var nearestPlayer *Player
	minPlayerDist := 99999.0
	for _, p := range players {
		if p.RespawnTimer > 0 {
			continue
		}
		dist := math.Sqrt((p.X-b.X)*(p.X-b.X) + (p.Y-b.Y)*(p.Y-b.Y))
		if dist < minPlayerDist {
			minPlayerDist = dist
			nearestPlayer = p
		}
	}

	// Decide target
	var targetX, targetY float64
	chasingOrb := false
	if nearestOrb != nil {
		// Always prioritize orbs over players
		targetX = nearestOrb.X + OrbSize/2
		targetY = nearestOrb.Y + OrbSize/2
		chasingOrb = true
	} else if nearestPlayer != nil {
		targetX = nearestPlayer.X + PlayerSize/2
		targetY = nearestPlayer.Y + PlayerSize/2
	}

	input := &InputState{}
	if rand.Intn(120) == 0 {
	input.Dash = true
	log.Printf("Bot %d DASH!", b.Id)
	}

	// Try pathfinding if we have a target
	if targetX != 0 || targetY != 0 {
		botCX := b.X + PlayerSize/2
		botCY := b.Y + PlayerSize/2

		needNewPath := len(b.Path) == 0 || now-b.LastPathTime > 1.0 || b.StuckTimer > 0.5

		if !needNewPath && b.PathIndex < len(b.Path) {
			wp := waypoints[b.Path[b.PathIndex]]
			distToWP := math.Sqrt((botCX-wp.X)*(botCX-wp.X) + (botCY-wp.Y)*(botCY-wp.Y))
			if distToWP < 80 {
				b.PathIndex++
				if b.PathIndex >= len(b.Path) {
					needNewPath = true
				}
			}
		}

		if needNewPath {
			bestPath := findPath(botCX, botCY, targetX, targetY)
			if len(bestPath) == 0 {
				leftPath := findPath(botCX-80, botCY, targetX, targetY)
				rightPath := findPath(botCX+80, botCY, targetX, targetY)
				if len(leftPath) > 0 && (len(rightPath) == 0 || len(leftPath) < len(rightPath)) {
					bestPath = leftPath
				} else if len(rightPath) > 0 {
					bestPath = rightPath
				}
			}

			samePath := len(bestPath) == len(b.Path)
			if samePath {
				for i := range bestPath {
					if bestPath[i] != b.Path[i] {
						samePath = false
						break
					}
				}
			}

			b.Path = bestPath
			b.LastPathTime = now
			b.StuckTimer = 0
			if !samePath || b.PathIndex >= len(bestPath) {
				b.PathIndex = 0
			}
		}

		if len(b.Path) == 1 && b.StuckTimer > 0.5 {
			b.Path = nil
		}

		dx := math.Abs(b.X - b.LastPathX)
		dy := math.Abs(b.Y - b.LastPathY)
		if dx < 0.3 && dy < 0.3 {
			b.StuckTimer += 1.0 / 60.0
		} else {
			b.StuckTimer = 0
		}
		b.LastPathX = b.X
		b.LastPathY = b.Y

		if len(b.Path) > 0 {
			if b.PathIndex >= len(b.Path) {
				b.PathIndex = len(b.Path) - 1
			}
			wp := waypoints[b.Path[b.PathIndex]]
			distToWP := math.Sqrt((botCX-wp.X)*(botCX-wp.X) + (botCY-wp.Y)*(botCY-wp.Y))

			if distToWP < 50 && b.PathIndex+1 < len(b.Path) {
				nextWP := waypoints[b.Path[b.PathIndex+1]]
				input = moveToward(b, nextWP.X, nextWP.Y, now, chasingOrb)
			} else if wp.Y > botCY+40 && b.OnGround {
				if wp.X < botCX {
					input.Left = true
				} else {
					input.Right = true
				}
				input.Jump = false
			} else if distToWP < 50 {
				input = moveToward(b, targetX, targetY, now, chasingOrb)
			} else {
				input = moveToward(b, wp.X, wp.Y, now, chasingOrb)
			}
		} else {
			if targetY < b.Y-80 && !platformAbove(b) {
				if b.Facing > 0 {
					input.Right = true
				} else {
					input.Left = true
				}
				if b.X < 50 {
					b.Facing = 1
				} else if b.X > WorldWidth-80 {
					b.Facing = -1
				}
			} else if targetY > b.Y+30 && b.OnGround {
				if targetX < botCX {
					input.Left = true
				} else {
					input.Right = true
				}
				input.Jump = false
			} else {
				input = moveToward(b, targetX, targetY, now, chasingOrb)
			}
		}
	}

	inputs[b.Id] = input
	applyPhysics(&b.Player, input, players, b.Id, now)
}