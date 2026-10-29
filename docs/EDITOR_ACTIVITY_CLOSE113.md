# Editor activity and close policy (113 candidate)

## Loaded-document badge

The existing Text Editor sidebar activity indicator includes saved and unsaved
loaded documents in the main window and detached editor windows. Its tooltip and
accessible label give the loaded-document count. An untouched empty Untitled tab
is excluded; an empty file opened from storage and an edited draft subsequently
undone to empty are included. Closing the last document clears the indicator.

The authenticated renderer reports only bounded `{uid, lease, loaded}` metadata
to `POST /api/editor-views/current/activity` (maximum 64 entries and 16 KiB).
The existing `GET /api/activity` remains read-only and returns numeric counts, not
UIDs, paths, text or credentials. Canonical registry ownership takes precedence;
a document is counted once during offer, staging and accepted transfer. Stale
owners/leases are rejected. Confirmed renderer exit/release removes renderer-only
metadata. Existing last-acknowledged recovery documents remain counted.

Updates are coalesced and follow the existing activity polling cadence. Metadata
failure cannot block terminal/server activity reads. Reporting requires an
explicit authenticated-online availability callback, rechecked at dispatch;
it does not require native-window launch support in the main browser. Detached
reporting additionally retains the existing scoped-window capability guard.
This is in-memory activity, not new durable plaintext document storage or crash
recovery. An abrupt renderer/network failure can retain its last known count
until confirmed lifecycle cleanup; heartbeat silence is not proof of destruction.

## Ask before discarding unsaved changes

The existing editor More disclosure contains **Confirm unsaved close** (accessible name: **Ask to save unsaved changes before closing**), enabled by default. Main-window changes persist with the existing
editor preferences; unrelated wrap/font/theme changes preserve explicit false.
Turning it off requires an explicit **Allow discard** warning. Detached editors
receive a read-only policy mirror from the main window; they cannot write app
settings. Missing or unavailable policy is treated as enabled.

Disabling the preference skips only ordinary dirty-discard consent for tab close,
Close all and an approved main-window shutdown. It does not delete files, autosave
or bypass pending save, composition, transfer, ownership or shutdown
acknowledgement guards. The existing native window preparation also refuses
pending loading, reloading and formatting. A failed save remains visible and
forces an explicit discard confirmation even when the preference is off. The boolean
failed-save marker follows in-memory document transfers; error text is not added
to the transfer schema. Successful save/reload clears it.

Native detached-window close still returns its documents through the existing
main-window offer/accept/ACK path; it does not silently discard them. Move and
merge always preserve document state, independent of this preference. Main close
preparation remains nondestructive; cancel or uncertain/rejected shutdown does
not commit document removal. The existing safe cancellation of an ordinary
tab/Close all during a read or format is preserved; late completions cannot revive the closed document.

## Validation and limits

Focused Go race tests cover unique counts across real registry transfers,
lease-zero registration cleanup, stale-owner rejection, empty saved/draft cases,
renderer lifecycle cleanup, strict metadata bounds/schema, concurrent reads and
updates, default/explicit-false policy and denied child settings mutation.
JavaScript tests cover policy persistence, cancellation, unconditional busy
checks, failed-save warnings, nondestructive close preparation, metadata
allowlisting/coalescing/availability loss and accessible count removal.

`scripts/editor-activity-close113.browser.cjs` exercises actual production UI with
synthetic data at 1440 and 390 px and a detached policy mirror. It uses
`PLAYWRIGHT_MODULE` and `TOOLKIT_TEST_OUTPUT`. The separately owned actual Windows
editor fixture covers native close cancellation, continued editing, explicit
false, active save/failure, pending transfer and child ACK return. Hosted browser
pixels and native execution must pass on the final integrated source; authored
fixtures and local unit tests are not evidence of those runtime passes. No local
user desktop, real user document, credential or device is used by these tests.

Local candidate verification: 1,129 JavaScript tests and 42 Python guards passed;
Go 1.26.10 race tests passed for the application, terminal, desktop and vaultcrypto
packages. Browser/native execution and final integrated aggregate remain required.
