# Network Toolkit 1.17.126 — administrative inventory repair

Scope: correct actual Windows read-only inventory failures that left the 125
Enable/Disable actions unavailable. No unrelated UI, action or adapter policy change.

Published125 was tested unchanged in diagnostic run41110299103,
source4f200e20c255f571f6b8537defc079b786e4a8e5, tree750616e1ccd53d1e8ebc6e5714c2cc63a6d3e1e0.
The exact production query timed out at5035ms. The identical fixed query with
separate streams returned valid JSON in13080ms:8 rows,8 matching GUID/index pairs,
4 manageable interfaces. Numeric administrative1/2 mapping was correct. There
were681bytes of module-progress CLIXML on stderr, separate from1638bytes JSON.
This reproduces a real production defect, not a mock-only result. It does not
prove the exact Windows timing on the user's device. No network mutation ran.

Correction: separate bounded stdout/stderr, suppress read-only module progress,
20-second query and 25-second IP inventory/preflight budgets within the unchanged
30-second HTTP write deadline. Hosts stays 8 seconds. Successful toggle refresh
gets 25 seconds; cancelled toggle and unrelated refresh limits stay unchanged.
Local focused tests pass: all TestLocal 0.368s; 15 network race groups 1.079s;
Windows cross-compilation. Actual fixed read-only Windows execution is pending
the exact-source hosted gate; this is not a claim that it has passed.
Unknown/conflicting/locked/unsupported interfaces remain fail-closed; one-shot
UAC, identity/revision checks, explicit review and no automatic retry remain.
Existing unrelated editor-transfer/comparison/resize/Hosts observations remain.
Keep all125 and older published tags/assets immutable. Actual user/CI adapter
Enable/Disable and a new visible UAC consent transition are not authorized tests.
