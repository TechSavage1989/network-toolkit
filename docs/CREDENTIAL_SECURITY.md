# Credential security — 1.17.104 development checkpoint

This document describes the changes being developed and their verification
requirements. It is not a claim that the current published 1.17.103 binary has
these protections, nor a high-assurance certification.

## Implemented in the development branch

- Windows HTTP pages no longer disclose full application/view capabilities.
  The native host injects an in-memory capability before navigating to its exact
  trusted top-level document. Bootstrap registration failure stops startup.
  The native host grants only its initial exact-document navigation. After that
  one-shot permit is consumed, full document reloads, nested documents and new
  full-page navigations are denied before network access. In-app panels continue
  to work; close and reopen the native window for a fresh document/bootstrap.
  Independent Terminal activation records contain a separate activation-only
  capability, not a key to credential or other application APIs.
- Saved passwords and private keys never return from credential GET endpoints.
  Metadata-only lists/editors submit credential IDs for approved backend use.
  Replacement secret input is write-only; blank edits keep the existing secret.
- Credential use is bound to an explicit protocol, hostname/IP, port and username.
  Adding a destination requires a native default-No confirmation. SSH identities
  are pinned in the encrypted credential record; FTPS uses the approved pin or
  ordinary certificate/hostname validation, never a per-use UI trust override.
  Verify displayed fingerprints through a trusted independent source. A network
  inspection or previously saved key is not proof of server ownership by itself.
- Legacy credentials have no bindings and cannot silently acquire them. Approval
  is an explicit action. No passwords, device settings or user files are migrated
  or changed merely by publishing this code.
- Saved credentials are not allowed for Telnet or unencrypted FTP.
- Reconnect retains safe settings and an ID, not plaintext passwords/private keys
  or passphrases. It resolves saved credentials again. Manual password/key users
  must re-enter authentication. Existing authenticated SFTP transport adoption
  remains possible without caching its original key material.
- Non-Windows builds fail closed for the credential vault. Synthetic tests inject
  an isolated encrypted provider; there is no runtime plaintext fallback switch.
- New encrypted vault files are created with a protected Windows DACL for the
  current user, SYSTEM and Administrators, using an exclusive, non-inheritable
  handle. Creation never opens an existing file or changes its permissions.
  File/link and size checks reject
  unsafe or unreadable storage instead of silently downgrading protection.
- Vault size/quota rejection rolls back; authenticated FTP rejection text is not
  echoed because a server could include the submitted password in its reply.

## Master-password vault and remembered unlock

The v2 implementation uses AES-256-GCM with a fresh random 256-bit vault key.
Argon2id (64 MiB, three iterations, one lane, random 16-byte salt) derives the
password wrapping key. All entries, names, usernames, destination bindings and
saved secret fields are encrypted together. Only format/KDF/cryptographic framing
metadata is outside encryption, authenticated against modification. Passwords
are exact UTF-8 without trimming or normalization: new passwords need at least
12 Unicode characters and at most 1024 UTF-8 bytes. Expensive KDF parameters,
message sizes and concurrent KDF work are bounded before derivation.

The user explicitly chose remembered unlock across application restarts and
Windows reboots after the security tradeoff was explained. The default native
Remember option saves a CurrentUser DPAPI-protected random vault key, never the
master password. It is bound to vault identity/generation and a backend-owned
configuration identity. **Software running as the same Windows account can
access remembered unlock.** A master password does not prevent that while a
usable remembered record exists. Unchecking Remember gives session-only unlock.
No automatic idle/workstation-lock prompt is enabled. Manual Lock and forget
clears current key access, removes remembrance and invalidates other instances
on their next credential use/status check. It preserves established authenticated
connections but blocks new saved-credential uses and reconnects.

Master passwords and optional recovery keys are entered in native Windows
controls, not web inputs or HTTP request bodies. Setup/rotation can offer a fresh
random offline recovery key, displayed only in the native dialog and requiring
full re-entry plus acknowledgment before activation. It is an alternate unlock
secret: store it separately and securely. There is no vendor recovery. Losing all
usable passwords/recovery keys and remembered unlock means losing vault access.
Changing password or recovering rotates the key and identity; old password,
recovery and remembered records no longer unlock the new vault. This does not
make old copied vault versions unreadable to their old keys.

## Storage, migration and reset

Files are under the selected application-data directory, alongside app settings:

- `Network_Toolkit.credentials.v2.vault`: authenticated master-password envelope.
- `Network_Toolkit.credentials.remember`: optional DPAPI-protected remembered key.
- `Network_Toolkit.credentials.vault`: retained legacy DPAPI-only vault, if any.

A persistent private lock file serializes credential transactions across app
instances. Expected-file checks reject stale updates rather than overwriting a
newer vault. The ordinary settings save cannot supply or restore the backend-owned
configuration identity from renderer JSON.

An existing legacy vault is **not automatically opened or migrated**. Explicit
native setup reads it, validates the new encrypted candidate with the entered
password and optional recovery key, writes/readbacks a protected temporary file,
and atomically activates the new version. Cancellation/failure before activation
keeps the existing copy unchanged. A corrupt v2 file never silently falls back
to v1. The old DPAPI copy remains a weaker-protected backup until the user invokes
separate native-confirmed cleanup after verifying migration and recovery.

Configuration reset/deletion forgets remembered unlock but retains encrypted
vaults. Moving/copying app data does not copy remembered unlock. Deleting one
credential or connection never deletes the entire vault. Deleting the current
master-password vault is a separate native default-No confirmation; any retained
legacy copy stays setup-only, never silently reactivated. Backup copies may
retain old credentials or unlock keys; manage them separately. File deletion is
not guaranteed secure erasure of disk blocks, SSD remnants or external backups.

## Important limits

The browser still holds the full app capability and can use legitimate local
cmd/PowerShell tools. A fully compromised renderer can reach same-user native
execution. This removes unnecessary secret exposure and unauthenticated local
token harvesting; it is not a strong isolation boundary against a compromised
renderer, same-user malware, administrator, kernel or operating system.
Native confirmation is not a substitute for cryptographic authentication.

New manually entered connection credentials still pass transiently through web
input fields. Only vault master/recovery secrets use native entry. Manual SSH
input caches clear after successful authentication; manual FTPS still needs its
transient password for stateless operations. Ordinary memory copies, paging and
crash dumps are not guaranteed securely erased. Established remote connections
remain authorized when a credential is locked/deleted; revoke remote credentials
on the server when necessary. Terminal transcripts/exports can contain secrets
printed by remote programs. No automatic transcript redaction can identify every
possible secret. This is not an independent security audit or certification.

Whole-directory rollback is not cryptographically prevented: restoring an old
vault, old settings identity and its remembered record can restore the old state.
Cryptographic authentication detects tampering, not every replay of a valid old
backup. Remote credential rotation and sensible backup retention remain necessary.

## Required verification

Build maintenance is part of this work. The release toolchain is pinned to the
patched, supported Go 1.26.10 series (released 2026-10-19), rather than the old
unsupported Go 1.23.2 pin. This preserves the Windows 10/x64 platform minimum and
avoids taking Go 1.27's additional default JSON and platform-certificate-store
behavior changes in the same security build. See the official
[Go release history](https://go.dev/doc/devel/release) and
[Go 1.27 notes](https://go.dev/doc/go1.27).

The dependency gate verifies the existing pinned module sums, regenerates vendor
content for comparison, and runs pinned govulncheck v1.9.0 against the public Go
vulnerability database for both Linux and Windows build configurations. A clean
scan is evidence about known reachable vulnerabilities, not an independent audit
or a guarantee against unknown vulnerabilities. Report scanner/database failures
and findings rather than treating them as a pass. No dependency upgrade is implied
by a formatting or vendor-metadata repair.

Canonical comparison also found six historical Go-compatibility edits in the
vendored SSH package. The development branch restores those files byte-for-byte
from checksum-verified x/crypto v0.58.0: upstream FIPS-mode checks and supported
hybrid ML-KEM key exchange are no longer patched out. The module versions are
unchanged. This does not enable system FIPS mode or certify Toolkit as FIPS compliant.

At the 2026-10-27 development checkpoint, both source configurations reported
zero reachable-symbol or imported-package vulnerabilities. The module-level
[GO-2026-5932 advisory](https://pkg.go.dev/vuln/GO-2026-5932) concerns the
unmaintained `golang.org/x/crypto/openpgp` packages, which Toolkit neither imports
nor vendors. It has no fixed version; this is not a claim that every package in
the required module is vulnerability-free. Re-run the gate on the final source.

Run the normal Checks pipeline and added Windows security steps. Windows tests
must exercise real DPAPI round trips/tamper detection, inspect the actual DACL,
verify activation-only isolation, and load both root/detached pages in real WebView2.
The native bootstrap test requires NETWORK_TOOLKIT_NATIVE_SECURITY_TEST=1 and
fails, rather than skips, when the required runtime is unavailable.

Native document-request interception requires WebView2's ICoreWebView2_22 API.
Microsoft documents Runtime 122.0.2365.46 as the compatibility minimum for its
[SDK 1.0.2365.46 APIs](https://learn.microsoft.com/en-us/microsoft-edge/webview2/release-notes/sdk/1-0-2365-46).
This is only an API floor, not a recommendation to run that old runtime: install
the latest security-updated Evergreen x64 runtime from the
[official Microsoft download page](https://developer.microsoft.com/microsoft-edge/webview2/#download).
An unsupported runtime receives a clear update-required startup error; the app
does not install anything or fall back to weaker filtering. Windows 10/11 x64
remains the application platform requirement.

`scripts/credential-security.browser.cjs` uses isolated intercepted synthetic
responses to check UI behavior/layout only; it does not establish native approval,
DPAPI, operating-system account isolation or production backend security.
The old live Linux `connection-settings.browser.cjs` entrypoint is retired: it
relied on the removed plaintext fallback and cannot establish the new native
boundary. Do not re-enable that fallback to make the old test pass.

Final release additionally requires the combined graph normal/narrow rendered
30-second-reference checks, exact-source passing builds, review of all security
failure-path tests, and honest documentation of any remaining acceptance limits.
