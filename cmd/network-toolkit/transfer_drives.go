package main

// Inventory contains presentation metadata only. Volume-label lookup is bounded
// and cached; network volumes are never probed for labels. Selecting a destination
// still uses the normal checked listing.
type localTransferDrive struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
