package main

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Positions in the already-transferred screen, never raw keyboard input.
// This state is held by the live view registry and is not application settings.
type terminalOutputEcho struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type terminalOutputBlock struct {
	ID      string                 `json:"id"`
	Kind    string                 `json:"kind"`
	Start   int                    `json:"start"`
	End     int                    `json:"end"`
	Closed  bool                   `json:"closed"`
	At      int64                  `json:"at"`
	Echo    []terminalOutputEcho   `json:"echo"`
	Archive *terminalOutputArchive `json:"archive,omitempty"`
}
type terminalOutputArchive struct {
	Screen    string `json:"screen"`
	Echo      []int  `json:"echo"`
	Closed    bool   `json:"closed"`
	Truncated bool   `json:"truncated"`
}
type terminalSessionOutputSettings struct {
	AutoPager       *bool  `json:"autoPager,omitempty"`
	AutoScroll      *bool  `json:"autoScroll,omitempty"`
	PasteReview     *bool  `json:"pasteReview,omitempty"`
	IncludeCommands *bool  `json:"pasteIncludeCommands,omitempty"`
	PagerKey        string `json:"pagerKey,omitempty"`
}
type terminalInputPosition struct {
	Line   int `json:"line"`
	Offset int `json:"offset"`
}
type terminalOutputBlocks struct {
	Version  int                           `json:"version"`
	Choice   string                        `json:"choice"`
	Settings terminalSessionOutputSettings `json:"settings"`
	Blocks   []terminalOutputBlock         `json:"blocks"`
	Input    *terminalInputPosition        `json:"input,omitempty"`
}

func (s *terminalOutputBlocks) validate() error {
	if s == nil {
		return nil
	}
	if s.Version != 1 || len(s.Choice) > 128 || len(s.Blocks) > 50 {
		return errors.New("The terminal output history is invalid or too large.")
	}
	if s.Input != nil && (s.Input.Line < 0 || s.Input.Line > 10300 || s.Input.Offset < 0 || s.Input.Offset > 8192) {
		return errors.New("The terminal input position is invalid.")
	}
	if s.Settings.PagerKey != "" && s.Settings.PagerKey != "auto" && s.Settings.PagerKey != "space" && s.Settings.PagerKey != "enter" {
		return errors.New("The terminal pager key is invalid.")
	}
	seen := map[string]bool{}
	archiveBytes := 0
	for _, b := range s.Blocks {
		if b.ID == "" || len(b.ID) > 128 || seen[b.ID] || (b.Kind != "command" && b.Kind != "paste") || b.Start < -1 || b.Start == -1 && b.Archive == nil || b.Start > 10300 || b.End < -1 || b.End > 10300 || b.Closed && b.End < b.Start || b.At < 0 || len(b.Echo) > 1024 {
			return errors.New("A terminal output block is invalid.")
		}
		seen[b.ID] = true
		if b.Archive != nil {
			archiveBytes += len(b.Archive.Screen)
			if archiveBytes > 4<<20 || !utf8.ValidString(b.Archive.Screen) || len(b.Archive.Echo) > 2048 {
				return errors.New("The retained command output exceeds its transfer limit.")
			}
			lines := strings.Count(b.Archive.Screen, "\n") + 1
			for _, line := range b.Archive.Echo {
				if line < 0 || line >= lines {
					return errors.New("A retained command echo position is invalid.")
				}
			}
		}
		for _, e := range b.Echo {
			if e.Start < b.Start || e.End < e.Start || e.End > 10300 {
				return errors.New("A terminal command echo position is invalid.")
			}
		}
	}
	return nil
}
func (s *terminalOutputBlocks) clone() *terminalOutputBlocks {
	if s == nil {
		return nil
	}
	c := *s
	if s.Input != nil {
		position := *s.Input
		c.Input = &position
	}
	c.Blocks = append([]terminalOutputBlock(nil), s.Blocks...)
	for i := range c.Blocks {
		c.Blocks[i].Echo = append([]terminalOutputEcho(nil), s.Blocks[i].Echo...)
		if s.Blocks[i].Archive != nil {
			value := *s.Blocks[i].Archive
			value.Echo = append([]int{}, value.Echo...)
			c.Blocks[i].Archive = &value
		}
	}
	// Settings are immutable values after validation. Copy pointed-to booleans too.
	for _, p := range []**bool{&c.Settings.AutoPager, &c.Settings.AutoScroll, &c.Settings.PasteReview, &c.Settings.IncludeCommands} {
		if *p != nil {
			v := **p
			*p = &v
		}
	}
	return &c
}
