package main

import (
	"math"
)

type Waypoint struct {
	X        float64
	Y        float64
	Width    float64
	Platform *Platform
	Edges    []int // indices of connected waypoints
}

var waypoints []Waypoint
var waypointGraph [][]int // adjacency list

// Build waypoint graph from platforms
func buildWaypointGraph() {
	waypoints = make([]Waypoint, len(platforms))
	for i := range platforms {
		waypoints[i] = Waypoint{
			X:        platforms[i].X + platforms[i].Width/2,
			Y:        platforms[i].Y,
			Width:    platforms[i].Width,
			Platform: &platforms[i],
		}
	}

	// Connect waypoints that are reachable from each other
	waypointGraph = make([][]int, len(platforms))
	for i := range waypoints {
		for j := range waypoints {
			if i == j {
				continue
			}
			if canReach(waypoints[i], waypoints[j]) {
				waypointGraph[i] = append(waypointGraph[i], j)
			}
		}
	}
}

// Check if you can reach waypoint B from waypoint A
func canReach(a, b Waypoint) bool {
	dx := math.Abs(a.X - b.X)
	dy := b.Y - a.Y

	// Same platform or very close (touching)
	if dx < 120 && math.Abs(dy) < 150 {
		return true
	}

	// Small step up
	if dy < -10 && dy > -80 && dx < 250 {
		return true
	}

	// Jump up
	if dy < -80 && dy > -160 && dx < 400 {
		return true
	}

	// Drop down
	if dy > 10 && dy < 350 && dx < 500 {
		return true
	}

	// Walk horizontally
	if math.Abs(dy) < 60 && dx < 700 {
		return true
	}

	return false
}

// BFS to find shortest path from bot position to target
func findPath(fromX, fromY, toX, toY float64) []int {
	// Find nearest waypoint to bot
	startWP := nearestWaypoint(fromX, fromY)
	endWP := nearestWaypoint(toX, toY)

	if startWP == -1 || endWP == -1 {
		return nil
	}
	if startWP == endWP {
		return []int{endWP}
	}

	// BFS
	visited := make([]bool, len(waypoints))
	parent := make([]int, len(waypoints))
	for i := range parent {
		parent[i] = -1
	}

	queue := []int{startWP}
	visited[startWP] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == endWP {
			// Reconstruct path
			var path []int
			for c := endWP; c != -1; c = parent[c] {
				path = append([]int{c}, path...)
			}
			return path
		}

		for _, neighbor := range waypointGraph[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	return nil // No path found
}

func nearestWaypoint(x, y float64) int {
	best := -1
	bestDist := 99999.0
	for i, wp := range waypoints {
		// Check if point is above this platform (within reasonable range)
		dx := math.Abs(x - wp.X)
		dy := y - wp.Y
		if dx < wp.Width/2+100 && dy > -150 && dy < 50 {
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < bestDist {
				bestDist = dist
				best = i
			}
		}
	}
	return best
}

// Get the next waypoint target for a bot
func getNextWaypoint(b *Bot, path []int) (float64, float64, bool) {
	if len(path) == 0 {
		return 0, 0, false
	}

	// If path only has one waypoint, go directly to it
	if len(path) == 1 {
		return waypoints[path[0]].X, waypoints[path[0]].Y, true
	}

	// Find which waypoint we're closest to
	currentIdx := nearestWaypoint(b.X+PlayerSize/2, b.Y+PlayerSize/2)
	
	// If we're at the first waypoint, go to the second
	if currentIdx == path[0] && len(path) > 1 {
		return waypoints[path[1]].X, waypoints[path[1]].Y, true
	}
	
	// If we're somewhere in the middle, go to the next one after current
	for i, wp := range path {
		if wp == currentIdx && i+1 < len(path) {
			return waypoints[path[i+1]].X, waypoints[path[i+1]].Y, true
		}
	}

	// If we're not on any waypoint in the path, go to the first one
	return waypoints[path[0]].X, waypoints[path[0]].Y, true
}