# Terminal paging and output boundaries

This is a development fix for the combined 1.17.104 work, not a published
release or a claim of universal hardware compatibility.

## Reproduced cause

The supplied 520 × 136 screenshot was materialized and visually inspected.
Its configuration output includes `interface 1/1/18` and `vlan access 30` and
ends with this complete prompt:

```
-- MORE --, next page: Space, next line: Enter, quit: q
```

The old matcher required a short `--More--` token at the end of a 512-character
raw tail. The explanatory suffix prevented a match. Stripping only complete
ANSI sequences did not handle split sequences, backspace erasure, or carriage
return redraw correctly. A second 800 ms signature throttle could suppress the
next legitimate identical page. Manual `q` and other pager interaction could
also create a new command draft and prematurely close the previous output block.

## Changes

- A per-session incremental recognizer tracks only one bounded visible logical
  line. It handles split UTF-8, CSI/OSC/control strings, SGR, carriage returns,
  backspaces and common erase/cursor operations. xterm remains authoritative
  for rendering, output parsing and selection. Unsupported movement is treated
  conservatively rather than turning arbitrary suffix text into a pager.
- Full-line known More forms include the supplied Aruba CX help suffix,
  ArubaOS-Switch/ProCurve Control-C suffix, Cisco IOS and ASA forms, and Junos
  More/percentage forms. Automatic mode sends Space for page-based forms;
  explicit next-line/next-page Enter prompts use Enter. Session Space/Enter
  overrides are preserved. No terminal length/page configuration command is sent.
- One advance is allowed per visible pager episode, not per time window.
  A 35 ms settling interval lets fragmented suffixes arrive. State, settings,
  macro ownership and input eligibility are rechecked before sending. A manual
  pager response cancels the pending automatic advance.
- Manual Space, Enter, q, Ctrl+C and pager help/navigation input pass through
  without creating a command draft or editable-command selection anchor. Single
  pager-key pastes also remain part of the current output block. Ordinary typed
  and keyboard/mouse pasted commands retain their existing separate boundaries.
- Known pager-only lines are omitted from output-only selection/copy. All output
  pages remain in the existing parsed archive even after console scrollback
  trimming. Selection falls back to that archive when a continuous xterm range
  cannot represent the complete output. Echo markers also validate their visible
  text so an ANSI redraw over an old command row does not silently hide output.
- Restored screens seed manual pager recognition without replaying an automatic
  key that the old window may already have queued. Prompt cursor spacing is
  retained in delayed-echo and paste archives.
- A non-echoing paste's initial idle prompt is excluded without storing hidden
  input or treating its first output line as a command echo.

## Safety and compatibility limits

Only complete recognized pager lines are advanced. Ordinary `more` prose,
embedded marker descriptions, yes/no prompts, and destructive confirmations do
not match. Bare `Press Enter to continue` and `Press space to continue` are
ambiguous and no longer trigger automatic input; explicit paging or quit context
is required. Unknown/localized/custom prompts require manual paging and a
sanitized transcript before another narrowly tested pattern is added. A CLI can
print text identical to a pager; no text-only recognizer can prove device intent.
A delayed ambiguous suffix beyond the settling interval is likewise not a
protocol-level guarantee. No claim covers every device or firmware.

This change does not disable existing macro pager ownership or weaken credential,
reconnect, bootstrap or native-host safeguards. No actual switches, saved
credentials or live device commands were used in development tests.

The current archive policy is unchanged: the newest 50 command/paste blocks are
transient in the session; normal console scrollback is separate. Live capture
has no new byte/row truncation in this fix and remains subject to available
memory and already-reported upstream/output-loss conditions. Cross-window
handoff carries a bounded 4 MiB archive payload; a shortened archive is explicitly
labelled, not claimed complete. Clearing a session or history eviction disposes
its corresponding archive. Lost or never-received output is not reconstructed.

## Evidence and verification

Primary vendor references checked 2026-10-27:

- [Aruba CX show lag example](https://arubanetworking.hpe.com/techdocs/AOS-CX/10.14/HTML/link_aggregation/Content/Chp_LAG/LACP_LAG_cmds/sho-lag.htm)
  includes the exact screenshot pager syntax.
- [ArubaOS-Switch 16.11 Basic Operation Guide](https://arubanetworking.hpe.com/techdocs/AOS-Switch/16.11/Aruba%20Basic%20Operation%20Guide%20for%20AOS-S%2016.11.pdf)
  documents the Space/Enter/Control-C form.
- [Cisco IOS XE CLI basics](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/fundamentals/configuration/xe-16/fundamentals-xe-16-book/cf-cli-basics.html)
  documents the More prompt and Space/Return behavior.
- [Cisco ASA CLI](https://www.cisco.com/c/en/us/td/docs/security/asa/asa92/configuration/general/asa-general-cli/ref-cli.html)
  documents the arrow-delimited More prompt and Space/Enter/q.
- [Junos CLI User Guide](https://www.juniper.net/documentation/us/en/software/junos/cli/cli.pdf)
  and [CLI environment settings](https://www.juniper.net/documentation/us/en/software/junos/cli/topics/topic-map/cli-environment-settings.html)
  document More, Space and Enter. Other generic fixtures are syntax cases, not
  additional vendor/hardware certifications.

Focused engine checks: `scripts/terminal-pager.test.cjs`,
`scripts/terminal-output.test.cjs`, `scripts/terminal-controls.test.cjs` and
`scripts/terminal-input.test.cjs`. These include every byte split of the supplied
and common prompts, control sequences, repeated pages, false positives,
no echo, manual keys, ANSI redraw, multiline paste and complete archived output.
The existing 90,000-line/>16 MiB parsed-output test remains intact.

Browser acceptance: `scripts/terminal-pager.browser.cjs` loads actual application
assets and the unchanged production enqueue function against synthetic recording
transport. It covers native keyboard/right-click paste, native pager keys,
selection/clipboard, narrow wrapped output and scrollback eviction. The fixture
is **not** a live backend, device or native Windows test. Local Chromium startup
was blocked by sandbox IPC restrictions, and a scoped escalation failed before
execution. Syntax/fixture parsing are checked; actual browser acceptance must
run in the existing hosted review workflow before release:

```sh
PLAYWRIGHT_MODULE=/path/to/playwright \
NTK_BROWSER_OUTPUT_DIR=/path/to/results \
node scripts/terminal-pager.browser.cjs
```

The security release lead owns combined-source integration, exact-source CI,
review packaging and publication. This patch changes no version, main branch,
repository access, tag or release asset.

## Requested mouse/keyboard defaults

A second supplied screenshot was materialized and visually inspected. New or
unset preferences now use all its visible values: automatic selection copying
on, highlighted-value copying on, multiline-paste review on, right-click Paste,
Ctrl+C copies a selection and otherwise sends Ctrl+C, known-pager advancement
on, and Automatic pager key. The Ctrl+C dropdown's rendered text is clipped in
the image, but its visible wording matches that existing, uniquely named option.
Only autoPager and Ctrl+C differed from previous defaults.

Both the browser normalizer and backend settings/window-transfer normalizer
agree. Existing explicit false values, Always send Ctrl+C, and other saved
choices are preserved, including repeated normalization and state round trips.
Empty/reset preference objects use the new defaults; no new reset control,
connection or macro default is introduced. Session overrides remain independent.
`terminal-interaction-defaults.test.cjs` and `terminal_interaction_test.go` cover
new, partial, invalid, explicit, reset-normalization and persistence cases.

### Local patch validation (2026-10-27)

- Final `node --test scripts/*.test.cjs`: **744 passed, 0 failed**.
- Python repository/release guards: **42 passed**.
- Release metadata validation: **1.17.103**, intentionally unchanged while 104
  remains under combined development.
- Changed JavaScript/browser script syntax and `git diff --check`: passed.
- Baseline screenshot reproduction: `pagerPrompt` on e5a6a8c returned null for
  the exact supplied Aruba CX prompt; the new engine fixture recognizes it.
- Go execution was not available in this executor (`/usr/bin/go` is not the Go
  compiler; `go version` reports an unknown-option error). New/updated Go
  default/persistence tests require the lead's pinned-toolchain hosted Checks.
- Chromium acceptance was **not run successfully** locally. The new browser
  fixture, actual screenshots, clipboard and native key assertions still need
  hosted execution and pixel review; syntax checks are not browser acceptance.
