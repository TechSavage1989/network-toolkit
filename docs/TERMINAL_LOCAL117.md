# Local CMD and PowerShell sessions

The connection form accepts an existing absolute **Starting directory**, with a
read-only folder browser. An empty field means the current Windows account's home
folder. The folder is validated before launch and again in the shell process
boundary. It is passed as `CreateProcessW.lpCurrentDirectory`, never inserted into
`cd`, `pushd`, a command string, or shell arguments. Symlink/reparse destinations
are resolved to their real path. Missing, unreadable or non-directory paths fail
rather than silently falling back. CMD UNC starting directories are explicitly
unsupported; no drive mapping is created. Administrator-only unreadable folders
can fail the ordinary-user validation before UAC; choose a readable folder.

**Run as administrator** is off by default. A saved local profile may remember the
choice, but loading it, opening the dialog or polling status never launches a
process. Only Connect or an explicit Reconnect can request a new administrator
shell. Windows controls consent through normal `ShellExecuteExW` `runas`; this
option does not elevate the rest of Toolkit or change UAC/security settings.
Cancellation does not retry. The connection form keeps its settings on invalid
folder/UAC failure; Cancel during pending startup disconnects that attempt.

The **Admin** session badge and status use the actual shell process token, not the
checkbox. If Toolkit itself was separately launched elevated, even an unchecked
ordinary shell inherits that token and is correctly labelled. Existing live
session attach/detach retains the same transport; it does not request UAC again.
Explicit reconnect creates a new process and can prompt again.

## Native boundary

Only absolute Windows system CMD (`/d`) and Windows PowerShell (`-NoLogo`) paths
are permitted. No custom executable, startup command, bypass flag, credential or
file-operation request enters this helper. The dedicated helper role exits before
HTTP, settings, browser or vault initialization. It accepts a single session over
a random, first-instance, local-only named pipe with the existing user/admin/SYSTEM
ACL. Both peers validate retained process handles, exact PIDs and the same image.
Frames are versioned/strictly decoded and limited to 64 KiB; input/output is limited
to 16 KiB per message. After the startup grant, only interactive input, resize and
close are allowed. The elevated helper uses the same ConPTY transport and
kill-on-close job as ordinary local shells. Parent loss, pipe EOF or cancellation
closes the shell and ordinary descendants inherited by its job. This is an
interactive administrator shell under UAC, not a sandbox: intentional
administrator-created services, scheduled tasks or brokered processes are not
contained or rolled back by closing the session. Late consent after cancellation
finds no startup pipe/grant and cannot create a shell. No always-running elevated
service is added.

The directory browser is a directories-only authenticated GET under the existing
can-create-Terminal scope. Single-session detached children cannot browse; a ready
detached workspace may choose a folder. It cannot preview/read files, mutate them,
connect remotely, or request elevation.

## Verification and limits

Focused tests cover literal paths, default-off/remote-field exclusion, invalid
folder retention, strict framing, peer/ready checks and cancellation. Windows tests
exercise actual ordinary ConPTY cwd/token state and job cleanup; a launcher-spy
covers UAC cancellation/late completion without interacting with a consent screen.
An actual same-image elevated helper test runs only when the isolated CI runner is
already elevated and reports both parent/child token states. That is not proof of
a visible UAC dialog or an unelevated-to-elevated consent transition. Browser UI
fixtures use synthetic local-session APIs, never real user files or privileged
commands. No user desktop/UAC interaction was performed.

Official references (implementation interpretation, not copied vendor samples):
- [CreateProcessW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw)
- [SHELLEXECUTEINFO and runas](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow)
- [Creating a pseudoconsole session](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session)
- [TOKEN_ELEVATION](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-token_elevation)
