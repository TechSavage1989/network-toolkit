# Network Toolkit v1.17.142 Beta 6 - reliability and optimization

## Roadmap

- Beta 4: profile and optimize application-wide performance bottlenecks.
- Beta 5: standardize interface layouts, responsiveness and common workflows.
- Beta 6: prolonged-operation reliability, simultaneous tools, bounded memory,
  cancellation/recovery and cross-tool interaction before any stable promotion.

## Scoped Beta 6 improvements

| Area | Issue or risk | Change and evidence |
| --- | --- | --- |
| Monitoring/diagnostic graph | Application-wide DOM changes can schedule graph layout work during unrelated Terminal and Packet Capture updates | Only graph-relevant mutation records trigger graph reconciliation |
| Navigation and memory | Cached graph models could retain detached DOM and ResizeObserver targets after changing tools | Release detached nodes, graph/result references and stale inspection pointers; preserve scroll preferences |
| Cancellation and recovery | A divider drag could survive removal of its graph workspace | Cancel stale drag when detached; preserve intended scroll positions across legitimate replacement |
| Editor | CodeMirror virtual height/gutter disagreement in narrow comparison layout | Native line-number blank spacer markers and bounded height-recovery attempts; see known CI limitations |

## Synthetic acceptance

- Graph workspace targeted regression, including 1,200 unrelated mutation records
  in 30 repeated observer batches, real graph updates, replacing parent panels,
  detaching/reopening a workspace and cancellation during active drag.
- Existing Packet Capture cancellation/recovery and remote-browser stale-session
  regressions were run with Graph tests: 122/122 passed locally.
- Existing JS/Go/Python, browser, security, Windows and release workflows remain
  authoritative for the exact tagged source. Do not silently dismiss CI failures.

## Known limitations and follow-up

Physical Windows soak duration, process working-set growth, multiple live network
devices, packet-capture uptime and concurrent live SSH/SFTP/serial sessions were not
independently measured here. Beta 6 is not a stable certification.

The hosted Editor aligned-comparison and continuous narrow-map motion tests
have had intermittent failures even when targeted divider tests pass. Preserve
failure evidence and work on these independently in a subsequent beta if needed.
Do not alter the test thresholds to manufacture a pass.

Keep the stable executable name, update-channel opt-in, saved credentials,
permissions, captured-data semantics and user confirmation unchanged.
