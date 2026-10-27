# Credential security — 1.17.104 development checkpoint

This document describes the changes being developed and their verification
requirements. It is not a claim that the current published 1.17.103 binary has
these protections, nor a high-assurance certification.

## Implemented in the development branch

- Windows HTTP pages no longer disclose full application/view capabilities.
  The native host injects an in-memory capability before navigating to its exact
  trusted top-level document. Bootstrap registration failure stops startup.
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
- New encrypted vault files receive a protected Windows DACL for the current user,
  SYSTEM and Administrators before writing. File/link and size checks reject
  unsafe or unreadable storage instead of silently downgrading protection.
- Vault size/quota rejection rolls back; authenticated FTP rejection text is not
  echoed because a server could include the submitted password in its reply.

## Important remaining limits and pending choices

The current branch still uses the existing per-user DPAPI vault format and
unlocks it automatically at startup. A true master-password/protected-key vault,
offline recovery and automatic locking require the user's pending decisions.
Do not describe this checkpoint as implementing them.

The browser still holds the full app capability and can use legitimate local
cmd/PowerShell tools. Therefore a fully compromised renderer can reach same-user
native execution. This work removes unnecessary saved-secret exposure and stops
unauthenticated local token harvesting; it does not establish a strong isolation
boundary against a compromised renderer, same-user malware, administrator or OS.

New manually entered credentials still pass transiently through web input fields.
Native secret entry is a separate planned improvement. Ordinary memory copies,
OS paging and crash dumps are not guaranteed securely erased. An established
remote connection remains authorized even if its saved credential is deleted.

Deleting a record does not revoke remote credentials or erase old backups/disk
blocks. Legacy DPAPI-only copies retain their original protection. No irreversible
cleanup or credential rotation is automatic. Terminal transcript export captures
screen/scrollback and can contain secrets printed by remote programs.

## Pending cryptographic/locking design

Before implementation, choose a real master-password-derived or supported
protected-key unlock mechanism, a recovery policy, and whether locking disconnects
existing credential-backed sessions. A Hello/PIN consent dialog followed by ordinary
DPAPI decryption is not a cryptographic lock. Migration must be versioned and
transactional, explicitly initiated, verified before activation, and never silently
fall back to weaker old data. Recovery/backup retention and cleanup are explicit.

## Required verification

Run the normal Checks pipeline and added Windows security steps. Windows tests
must exercise real DPAPI round trips/tamper detection, inspect the actual DACL,
verify activation-only isolation, and load both root/detached pages in real WebView2.
The native bootstrap test requires NETWORK_TOOLKIT_NATIVE_SECURITY_TEST=1 and
fails, rather than skips, when the required runtime is unavailable.

`scripts/credential-security.browser.cjs` uses isolated intercepted synthetic
responses to check UI behavior/layout only; it does not establish native approval,
DPAPI, operating-system account isolation or production backend security.
The old live Linux `connection-settings.browser.cjs` entrypoint is retired: it
relied on the removed plaintext fallback and cannot establish the new native
boundary. Do not re-enable that fallback to make the old test pass.

Final release additionally requires the combined graph normal/narrow rendered
30-second-reference checks, exact-source passing builds, review of all security
failure-path tests, and honest documentation of any remaining acceptance limits.
