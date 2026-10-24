package desktop

import "testing"

func TestCloseHandshakeEventOrdering(t *testing.T) {
	tests := []struct {
		name         string
		events       []string
		wantRequests int
		wantCloses   int
	}{
		{"save finishes then shutdown cancels", []string{"close", "close", "handled", "cancel", "unhandled"}, 1, 1},
		{"already cancelled", []string{"cancel", "close"}, 0, 1},
		{"renderer unavailable", []string{"unavailable", "close"}, 0, 1},
		{"missing close hook", []string{"close", "unhandled", "cancel"}, 1, 1},
		{"renderer hangs", []string{"close", "close", "timeout", "handled", "cancel"}, 1, 1},
		{"hook acknowledges but server stalls", []string{"close", "handled", "close", "timeout", "unhandled"}, 1, 1},
		{"cancel beats callback", []string{"close", "cancel", "handled", "timeout"}, 1, 1},
		{"unexpected completion cannot close idle window", []string{"unhandled", "timeout"}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state closeHandshake
			requests, closes := 0, 0
			for _, event := range tt.events {
				var decision closeDecision
				switch event {
				case "close":
					decision = state.request(true, false)
				case "cancel":
					decision = state.request(true, true)
				case "unavailable":
					decision = state.request(false, false)
				case "handled":
					decision = state.completed(true)
				case "unhandled":
					decision = state.completed(false)
				case "timeout":
					decision = state.expired()
				}
				if decision == requestRendererClose {
					requests++
				}
				if decision == closeWindowNow {
					closes++
				}
			}
			if requests != tt.wantRequests || closes != tt.wantCloses {
				t.Fatalf("requests=%d closes=%d, want requests=%d closes=%d", requests, closes, tt.wantRequests, tt.wantCloses)
			}
		})
	}
}
