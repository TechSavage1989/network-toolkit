# Network Toolkit v1.17.138 Beta 2 - Phase 1 multi-pane stability (candidate)

## Purpose

Begin from published v1.17.137 Beta 1 and current main documentation-only
handoff. Focus Phase 1 Beta 2 on split-pane resizing and preserving a user's
scroll context. Keep the existing beta opt-in, stable channel, Windows portable
file name and verified update-package contract unchanged.

## Scoped implementation

- The terminal divider pointer handler now accumulates rapid movement and
  applies the last coordinate at a bounded animation-frame cadence. Pointerup
  commits the final coordinate even if the frame has not run. Pointer cancel,
  window focus loss or pointer-capture loss restores the pre-drag split ratio
  and clears pending work. Live grid geometry is read once per frame so changing
  window/sidebar dimensions during a drag do not cause stale ratios.
- Pane bodies and terminal display elements clip xterm canvas content while a
  resize is pending, keeping visual content within the assigned pane.
- When Follow Latest is off, beforeFit retains a viewport row as a fallback
  if the normal-buffer scroll marker disappears during xterm reflow. afterFit
  clamps that row to available scrollback. Epoch checks still protect a newer
  user scroll or Follow Latest action, and other panes are independent.
- The existing local xterm fitting, deferred cosmetic redraw, remote PTY
  resize sequencing, selected text, input/paste prompt guards, session
  ownership, detached windows, saved settings and updater are unchanged.

## Validation and release

Run new scripts/terminal-beta138-pane.test.cjs together with existing
terminal-fit/resize/output, terminal-pane-detach and full JS/Go/Python suite.
Use GitHub Checks to validate responsive browser and Windows CI, and the
existing exact-source Release workflow for Windows PE resource generation,
ZIP allowlist/integrity, release assets and updater metadata.

Build channel remains beta via docs/RELEASE_CHANNEL.txt and compiled
updateBuildChannel. Publish v1.17.138 as a GitHub prerelease, not stable.
Verify v1.17.136 remains GitHub stable latest, v1.17.137 is unchanged,
four uploaded assets have SHA-256, and the opt-in feed sees the beta.

Unrelated historical Editor alignment, Windows native SFTP and shell echo
CI issues must not be called fixed by this scoped change. The test fixtures
use synthetic pane and terminal behavior, not a physical SSH latency benchmark.

PUBLICATION PENDING.
