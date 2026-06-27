package main

import (
	"math"
	// "log"
)

// ─── Player Physics ───
func applyPhysics(p *Player, input *InputState, allPlayers map[int]*Player, myId int, now float64) {
	respawning := p.RespawnTimer > 0
	if respawning {
		p.RespawnTimer -= 1.0 / 60.0
		if p.RespawnTimer < 0 {
			p.RespawnTimer = 0
		}
	}

	// ─── Bot uses completely different physics ───
	if p.IsBot {
		applyBotPhysics(p, input, now)
	} else {
		applyPlayerPhysics(p, input, now)
	}

	// ─── Platform collision (shared) ───
	newX := p.X + p.Vx
	newY := p.Y + p.Vy

	newX = clamp(newX, 0, WorldWidth-PlayerSize)
	newY = clamp(newY, 0, WorldHeight-PlayerSize)

	if newX == 0 || newX == WorldWidth-PlayerSize {
		p.Vx = 0
	}
	if newY == WorldHeight-PlayerSize {
		p.Vy = 0
	}

	p.OnGround = false

	for _, plat := range platforms {
		if overlap(newX, p.Y, PlayerSize, PlayerSize, plat.X, plat.Y, plat.Width, plat.Height) {
			if p.Vx > 0 {
				newX = plat.X - PlayerSize
			} else if p.Vx < 0 {
				newX = plat.X + plat.Width
			}
			p.Vx = 0
		}
		if overlap(newX, newY, PlayerSize, PlayerSize, plat.X, plat.Y, plat.Width, plat.Height) {
			if p.Vy > 0 {
				newY = plat.Y - PlayerSize
				p.Vy = 0
				p.OnGround = true
			} else if p.Vy < 0 {
				newY = plat.Y + plat.Height
				p.Vy = 0
			}
		}
	}

	// ─── Collision with others (only when not respawning) ───
	if !respawning {
		for otherId, other := range allPlayers {
			if otherId == myId || other.RespawnTimer > 0 {
				continue
			}
			if overlap(newX, newY, PlayerSize, PlayerSize, other.X, other.Y, PlayerSize, PlayerSize) {
				overlapX := math.Min(newX+PlayerSize, other.X+PlayerSize) - math.Max(newX, other.X)
				overlapY := math.Min(newY+PlayerSize, other.Y+PlayerSize) - math.Max(newY, other.Y)
				if overlapX < overlapY {
					pushX := overlapX / 2
					if newX < other.X {
						newX -= pushX
						other.X += pushX
					} else {
						newX += pushX
						other.X -= pushX
					}
					p.Vx = 0
					other.Vx = 0
				} else {
					if newY+PlayerSize-other.Y < other.Y+PlayerSize-newY {
						newY = other.Y - PlayerSize
						p.Vy = 0
						p.OnGround = true
					} else {
						other.Y = newY - PlayerSize
						other.Vy = 0
						other.OnGround = true
					}
				}
			}
		}

		for i := range bots {
			other := &bots[i].Player
			if other.RespawnTimer > 0 {
				continue
			}
			// Skip if this is the same bot (comparing pointers)
			if p == other {
				continue
			}
			if overlap(newX, newY, PlayerSize, PlayerSize, other.X, other.Y, PlayerSize, PlayerSize) {
				overlapX := math.Min(newX+PlayerSize, other.X+PlayerSize) - math.Max(newX, other.X)
				overlapY := math.Min(newY+PlayerSize, other.Y+PlayerSize) - math.Max(newY, other.Y)
				if overlapX < overlapY {
					pushX := overlapX / 2
					if newX < other.X {
						newX -= pushX
						other.X += pushX
					} else {
						newX += pushX
						other.X -= pushX
					}
					p.Vx = 0
					other.Vx = 0
				} else {
					if newY+PlayerSize-other.Y < other.Y+PlayerSize-newY {
						newY = other.Y - PlayerSize
						p.Vy = 0
						p.OnGround = true
					} else {
						other.Y = newY - PlayerSize
						other.Vy = 0
						other.OnGround = true
					}
				}
			}
		}
	}

	p.X = newX
	p.Y = newY
}

// ─── Player-specific physics ───
func applyPlayerPhysics(p *Player, input *InputState, now float64) {
	isDashing := (now - p.LastDash) < 0.1

	if isDashing {
		p.Vx = p.Facing * DashForce
	} else if input.Left {
		p.Vx = -PlayerSpeed
		p.Facing = -1
	} else if input.Right {
		p.Vx = PlayerSpeed
		p.Facing = 1
	} else {
		p.Vx = 0
	}

	if input.Dash && (now-p.LastDash >= DashCooldown) {
		if input.Left {
			p.Facing = -1
		} else if input.Right {
			p.Facing = 1
		}
		p.Vx = p.Facing * DashForce
		p.LastDash = now
	}

	if input.Jump && p.OnGround {
		p.Vy = JumpForce
		p.OnGround = false
	}

	p.Vy += Gravity
}

// ─── Bot-specific physics (heavy, slow, deliberate) ───
func applyBotPhysics(p *Player, input *InputState, now float64) {
	// Heavier gravity
	p.Vy += BotGravity
	// log.Printf("BOT GRAVITY: Vy=%.2f BotGravity=%.2f", p.Vy, BotGravity)

	// Slow acceleration, high friction
	if input.Left {
		p.Vx -= 0.55
		if p.Vx < -BotSpeed {
			p.Vx = -BotSpeed
		}
		p.Facing = -1
	} else if input.Right {
		p.Vx += 0.55
		if p.Vx > BotSpeed {
			p.Vx = BotSpeed
		}
		p.Facing = 1
	} else {
		// Friction: slow down when no input
		if p.Vx > 0 {
			p.Vx -= BotFriction
			if p.Vx < 0 {
				p.Vx = 0
			}
		} else if p.Vx < 0 {
			p.Vx += BotFriction
			if p.Vx > 0 {
				p.Vx = 0
			}
		}
	}

	// Dash (weaker than players, shorter duration)
	botDashForce := 8.0
	botDashDuration := 0.1
	isDashing := (now - p.LastDash) < botDashDuration
	if input.Dash && (now-p.LastDash >= DashCooldown) {
		p.LastDash = now
		isDashing = true
	}
	if isDashing {
		p.Vx = p.Facing * botDashForce
		return
	}

	// Stronger jump
	if input.Jump && p.OnGround {
		p.Vy = BotJumpForce
		p.OnGround = false
	}
}

// ─── Thrown Orb Physics ───
func applyOrbPhysics(o *ThrownOrb) {
	var grav float64
	if o.OrbType == "up" {
		grav = GravityUp
	} else {
		grav = Gravity
	}
	o.Vy += grav

	newX := o.X + o.Vx
	newY := o.Y + o.Vy

	// Bounce off world boundaries
	if newX <= 0 {
		newX = 0
		o.Vx = math.Abs(o.Vx) * 0.5
	} else if newX >= WorldWidth-OrbSize {
		newX = WorldWidth - OrbSize
		o.Vx = -math.Abs(o.Vx) * 0.5
	}
	if newY <= 0 {
		newY = 0
		o.Vy = math.Abs(o.Vy) * 0.5
	} else if newY >= WorldHeight-OrbSize {
		newY = WorldHeight - OrbSize
		o.Vy = -math.Abs(o.Vy) * 0.5
	}

	o.OnGround = false

	for _, plat := range platforms {
		if overlap(newX, o.Y, OrbSize, OrbSize, plat.X, plat.Y, plat.Width, plat.Height) {
			if o.Vx > 0 {
				newX = plat.X - OrbSize
			} else if o.Vx < 0 {
				newX = plat.X + plat.Width
			}
			o.Vx *= -0.3
		}
		if overlap(newX, newY, OrbSize, OrbSize, plat.X, plat.Y, plat.Width, plat.Height) {
			if o.Vy > 0 {
				newY = plat.Y - OrbSize
				o.Vy = 0
				o.OnGround = true
			} else if o.Vy < 0 {
				newY = plat.Y + plat.Height
				o.Vy = 0
			}
		}
	}

	o.X = newX
	o.Y = newY
}

func resolvePlayerOverlaps() {
	ids := make([]int, 0, len(players))
	for id := range players {
		ids = append(ids, id)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a := players[ids[i]]
			b := players[ids[j]]
			if a.RespawnTimer > 0 || b.RespawnTimer > 0 {
				continue
			}
			if overlap(a.X, a.Y, PlayerSize, PlayerSize, b.X, b.Y, PlayerSize, PlayerSize) {
				overlapX := math.Min(a.X+PlayerSize, b.X+PlayerSize) - math.Max(a.X, b.X)
				overlapY := math.Min(a.Y+PlayerSize, b.Y+PlayerSize) - math.Max(a.Y, b.Y)
				if overlapX < overlapY {
					pushX := overlapX / 2
					if a.X < b.X {
						a.X -= pushX
						b.X += pushX
					} else {
						a.X += pushX
						b.X -= pushX
					}
					a.Vx = 0
					b.Vx = 0
				} else {
					if a.Y < b.Y {
						a.Y = b.Y - PlayerSize
						a.Vy = 0
						a.OnGround = true
					} else {
						b.Y = a.Y - PlayerSize
						b.Vy = 0
						b.OnGround = true
					}
				}
			}
		}
	}
}

func resolveBotOverlaps() {
	for i := 0; i < len(bots); i++ {
		for j := i + 1; j < len(bots); j++ {
			a := &bots[i].Player
			b := &bots[j].Player
			if a.RespawnTimer > 0 || b.RespawnTimer > 0 {
				continue
			}
			if overlap(a.X, a.Y, PlayerSize, PlayerSize, b.X, b.Y, PlayerSize, PlayerSize) {
				overlapX := math.Min(a.X+PlayerSize, b.X+PlayerSize) - math.Max(a.X, b.X)
				overlapY := math.Min(a.Y+PlayerSize, b.Y+PlayerSize) - math.Max(a.Y, b.Y)
				if overlapX < overlapY {
					pushX := overlapX / 2
					if a.X < b.X {
						a.X -= pushX
						b.X += pushX
					} else {
						a.X += pushX
						b.X -= pushX
					}
					a.Vx = 0
					b.Vx = 0
				} else {
					if a.Y < b.Y {
						a.Y = b.Y - PlayerSize
						a.Vy = 0
						a.OnGround = true
					} else {
						b.Y = a.Y - PlayerSize
						b.Vy = 0
						b.OnGround = true
					}
				}
			}
		}
	}
}