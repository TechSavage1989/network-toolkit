package model

import "time"

const Version = "1.17.122"

type Request struct {
	Tool    string         `json:"tool"`
	Target  string         `json:"target"`
	Options map[string]any `json:"options"`
}

type Event struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Result struct {
	Time    string `json:"time,omitempty"`
	Summary string `json:"summary"`
	Data    any    `json:"data,omitempty"`
}

type Emit func(Event)

func Log(emit Emit, level, message string, data any) {
	if emit != nil {
		emit(Event{Time: time.Now().UTC().Format(time.RFC3339Nano), Level: level, Message: message, Data: data})
	}
}
