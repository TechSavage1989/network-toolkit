# Network Toolkit 1.17.129 — terminal pane resize scheduling (candidate)

Not yet published. Exact-source local, hosted browser/native Windows, security
and package/release acceptance remain pending. This work starts from the128
candidate;128 publication and final CI acceptance are not established here.
Preserve published127 and all historical tags/assets and original observations.

## Reproduction and bounded correction

A synthetic 10ms movement sequence reproduced starvation in the previous trailing
60ms local-fit timer: resetting it repeatedly postponed xterm reflow until movement
stopped. A separate 20ms geometry sequence reproduced the same starvation in the
previous trailing120ms transport debounce. These are scheduler reproductions,
not a complete visual reproduction of the user's machine or live remote TUI.

Local fitting now retains the first pending16ms deadline and reads the latest
geometry when it runs. A per-terminal-view observer also detects pane/body changes
when the outer stage size does not change; it follows retained/reparented nodes
and disconnects with existing display disposables. Zero, infinite and otherwise
invalid geometry is skipped. Selection and scroll restoration remain protected.

PTY resize transport retains the first pending120ms deadline while replacing its
payload with the latest valid dimensions. Single-flight acknowledgments, session
epochs, no-op checks, ownership/restoration/transfer/page guards and prompt/output
revision safety remain. No-op acknowledged fits do not destabilize name detection.
Prompt-paced submission, Pause/Stop and uncertain-start safeguards are unchanged.

Local xterm reflow and remote application redraw are different: the latter depends
on acknowledged PTY resizing and the remote application. No synthetic remote
redraw or automatic input is sent by the application. Static text and arbitrary
remote programs cannot be promised universal reformatting or compatibility.

## Candidate validation scope and limits

Local scheduler/observer tests reproduce the timing defects and cover lifecycle,
invalid geometry and latest-payload behavior. Browser fixtures use real bundled
xterm normal-history width/height reflow, paused/follow-bottom scrolling and
alternate-buffer redraw/resize events. Intercepted browser transport proves local
acknowledgment plumbing, not actual remote application behavior. Real-timer
browser/native tests must remain real-timer tests; do not replace them with mocks.

Final aggregate counts and exact source/tree remain pending. Actual browser
screenshots/geometry at multiple layouts and repeated window/divider sizes,
detach/return and native Windows PTY/TUI resize evidence are pending. Local
Chromium is restricted by socket EPERM; no runtime pass is inferred from syntax
checks. No live SSH device, client configuration or universal TUI compatibility
is claimed. Tests/examples are synthetic; private screenshots/names/IPs stay out
of repository artifacts. Retain128 legacy-editor and earlier failed observations.

## Release boundary

Record exact-source final gate outcomes, package/source bytes, release tag/run,
actual public downloads/manifests/updater feed and prior-release immutability
before publication acceptance. The128 notes are archived byte-identically, not
rewritten as a publication claim. Reconcile128 closure only against actual proof.
This candidate changes no repository visibility/access, existing tags or assets.

