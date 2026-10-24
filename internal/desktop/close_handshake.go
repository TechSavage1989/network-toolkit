package desktop

type closeDecision uint8

const (
	waitForClose closeDecision = iota
	requestRendererClose
	closeWindowNow
)

// closeHandshake serializes user close requests and late renderer completions.
// Cancellation, startup failure and the deadline can always finish a close;
// a repeated title-bar click cannot start another state-save request.
type closeHandshake struct {
	pending  bool
	finished bool
}

func (c *closeHandshake) request(rendererReady, contextCancelled bool) closeDecision {
	if c.finished {
		return waitForClose
	}
	if contextCancelled || !rendererReady {
		c.finished = true
		return closeWindowNow
	}
	if c.pending {
		return waitForClose
	}
	c.pending = true
	return requestRendererClose
}

func (c *closeHandshake) completed(handled bool) closeDecision {
	if c.finished || !c.pending || handled {
		return waitForClose
	}
	c.finished = true
	return closeWindowNow
}

func (c *closeHandshake) expired() closeDecision {
	if c.finished || !c.pending {
		return waitForClose
	}
	c.finished = true
	return closeWindowNow
}
