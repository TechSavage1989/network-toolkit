package desktop

type closeDecision uint8

const (
	waitForClose closeDecision = iota
	requestRendererClose
	closeWindowNow
	closeNotApproved
)

// closeHandshake serializes user close requests and late renderer completions.
// Default hosts retain their bounded fallback. Approval-required ready hosts
// never infer consent from invocation or a deadline; repeated title-bar clicks
// cannot start another state-save request.
type closeHandshake struct {
	approvalRequired bool
	pending          bool
	finished         bool
}

func (c *closeHandshake) request(rendererReady, contextCancelled bool) closeDecision {
	if c.finished {
		return waitForClose
	}
	if contextCancelled || (!rendererReady && !c.approvalRequired) {
		c.finished = true
		return closeWindowNow
	}
	if !rendererReady {
		return closeNotApproved
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
	if c.approvalRequired {
		c.pending = false
		return closeNotApproved
	}
	c.finished = true
	return closeWindowNow
}

func (c *closeHandshake) expired() closeDecision {
	if c.finished || !c.pending {
		return waitForClose
	}
	if c.approvalRequired {
		c.pending = false
		return closeNotApproved
	}
	c.finished = true
	return closeWindowNow
}
