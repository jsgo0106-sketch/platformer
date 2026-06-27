package main

import (
	"math"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ─── World Constants ───
const (
	WorldWidth     = 2400
	WorldHeight    = 600
	Gravity        = 0.8
	PlayerSpeed    = 4
	JumpForce      = -15
	DashForce      = 14 // 12
	DashCooldown   = 0.3
	PlayerSize     = 30
	OrbSize        = 14
	OrbSpawnRate   = 3 // 7
	OrbFuseTime    = 3
	ExplosionWidth = 45
	PickupRange    = 45
	ThrowSpeedX    = 7 // 8
	ThrowSpeedY    = -7
	GravityUp	   = 0.2
	ThrowSpeedXUp  = 3
	ThrowSpeedYUp  = -11 // -12
	BotSpeed       = 4
	BotGravity     = 0.8
	BotJumpForce   = -15 // -15
	BotFriction    = 0.6 // 0.3
)

// ─── Platform ───
type Platform struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

var platforms = []Platform{
	// Full floor
	{0, 550, 2400, 50},

	// Left side - stairway to heaven
	{50, 490, 80, 15},
	{150, 430, 80, 15},
	{250, 370, 80, 15},
	{350, 310, 80, 15},
	{50, 250, 80, 15},

	// Left upper platforms
	{200, 200, 100, 15},
	{400, 160, 100, 15},

	// Left gap crossing (need to jump)
	{600, 480, 80, 15},
	{720, 420, 80, 15},
	{840, 360, 80, 15},

	// Center tower
	{1000, 480, 60, 15},
	{1050, 420, 60, 15},
	{1100, 360, 60, 15},
	{1150, 300, 60, 15},
	{1200, 240, 60, 15},

	// Tower side platforms
	{950, 350, 80, 15},
	{1280, 350, 80, 15},
	{950, 200, 80, 15},
	{1280, 200, 80, 15},

	// Center gap (big jump required)
	{1400, 480, 80, 15},
	{1520, 430, 80, 15},
	{1640, 380, 80, 15},

	// Mid-right floating islands
	{1500, 300, 100, 15},
	{1700, 260, 100, 15},
	{1900, 300, 100, 15},

	// Right side - descending steps
	{1900, 480, 80, 15},
	{2020, 430, 80, 15},
	{2140, 380, 80, 15},
	{2260, 330, 80, 15},

	// Right upper reward platforms
	{2000, 250, 100, 15},
	{2200, 200, 100, 15},
	{2100, 140, 120, 15},

	// Scattered small platforms
	{300, 450, 60, 15},
	{650, 350, 60, 15},
	{1350, 200, 60, 15},
	{1750, 400, 60, 15},
	{2300, 450, 60, 15},
}

// ─── Player ───
type Player struct {
	Id           int	 `json:"id"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Vx           float64 `json:"vx"`
	Vy           float64 `json:"vy"`
	OnGround     bool    `json:"onGround"`
	Color        string  `json:"color"`
	Facing       float64 `json:"facing"`
	LastDash     float64 `json:"lastDash"`
	DashCooldown float64 `json:"dashCooldown"`
	HasOrb       bool    `json:"hasOrb"`
	HeldOrbType  string  `json:"heldOrbType"` // "down" or "up"
	RespawnTimer float64 `json:"respawnTimer"`
	IsBot        bool    `json:"isBot"` // True for bots
}

type InputState struct {
	Left     bool   `json:"left"`
	Right    bool   `json:"right"`
	Jump     bool   `json:"jump"`
	Dash     bool   `json:"dash"`
	ThrowOrb bool   `json:"throwOrb"`
	Detonate bool   `json:"detonate"`
	ChatMsg  string `json:"chatMsg"`
}

// ─── Chat Message ───
type ChatMessage struct {
	PlayerId int     `json:"playerId"`
	Color    string  `json:"color"`
	Text     string  `json:"text"`
	Time     float64 `json:"time"`
}

// ─── Thrown Orb ───
type ThrownOrb struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Vx       float64 `json:"vx"`
	Vy       float64 `json:"vy"`
	FuseEnd  float64 `json:"fuseEnd"`
	OnGround bool    `json:"onGround"`
	OwnerId  int     `json:"ownerId"`
	OrbType	 string  `json:"orbType"` // "down" or "up"
}

// ─── World Orb ───
type Orb struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Timer    float64 `json:"timer"`
	OrbType	 string  `json:"orbType"` // "down" or "up"
}

// ─── Explosion ───
type Explosion struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Timer        float64 `json:"timer"`
	MaxTimer     float64 `json:"maxTimer"`
	IsHorizontal bool    `json:"isHorizontal"` // True for up, false for down
}

// ─── Bot ───
type Bot struct {
	Id               int     `json:"id"`
	Player
	TargetId         int     `json:"-"`
	ChangeTimer      float64 `json:"-"`
	LastJump         float64 `json:"-"`
	OrbPickupTime    float64 `json:"-"`
	OrbThrowTime     float64 `json:"-"`
	OrbThrown        bool    `json:"-"`
	DetonateReadyTime float64 `json:"-"`
	Path             []int   `json:"-"`
	PathIndex        int     `json:"-"`
	LastPathTime     float64 `json:"-"`
	LastPathX        float64 `json:"-"`
	LastPathY        float64 `json:"-"`
	StuckTimer       float64 `json:"-"`
}

// ─── Global State ───
var (
	players      = make(map[int]*Player)
	inputs       = make(map[int]*InputState)
	bots         []Bot
	orbs         []Orb
	thrownOrbs   []ThrownOrb
	explosions   []Explosion
	chatMessages []ChatMessage
	freeIds      []int
	nextId       = 1
	playersMu    sync.Mutex
	colors       = []string{"blue", "red", "green", "orange", "purple", "yellow", "cyan", "pink"}
	lastOrbSpawn float64
	connections  = make(map[int]*websocket.Conn)
	connMu       sync.Mutex
	upgrader     = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

// ─── Helpers ───
func overlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func clamp(x, min, max float64) float64 {
	return math.Max(min, math.Min(max, x))
}
