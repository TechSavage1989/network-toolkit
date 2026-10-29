package main

// Inventory reads mounted names only. It never opens a removable/network volume
// while rendering a drive picker; selecting one uses the normal checked listing.
type localTransferDrive struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
