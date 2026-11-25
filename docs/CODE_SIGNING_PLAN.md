# Code signing plan (Azure Artifact Signing)

Status: implemented on this branch, not yet active. The Release workflow signs
the Windows binaries with Azure Artifact Signing, gated on three repository
secrets that are not yet set. Nothing is signed until the secrets exist and a
signed beta is published and tested. This file records the Azure/GitHub setup
and how the signing works.

Known configuration (not secret), already in `.github/workflows/release.yml`:

- Endpoint: `https://eus.codesigning.azure.net` (East US)
- Account: `networktoolkit-signing`
- Certificate profile: `jpaull-codesign`

To confirm before the first signed run: the pinned version of Microsoft's
`sign` tool (`SIGN_TOOL_VERSION` in the workflow) against the latest on
nuget.org, and that `azure/login` leaves the OIDC environment the `sign` tool
reads. The first signed beta is the real end-to-end test.

## Why

SentinelOne's Behavioral AI deletes `Network_Toolkit.exe` on managed machines
because an **unsigned** program that runs **as administrator** and **hosts a
command shell** matches the behaviour of remote-access malware (confirmed on a
Windows Server 2022 test box, v1.21.84: the admin CMD tab is killed even when
the app is started elevated from Explorer, with no UAC request of our own). A
trusted Authenticode signature removes the "unsigned" leg of that pattern, is
the only fix that works on every machine without a per-machine exclusion, and
is what keeps Windows Terminal and PuTTY from being flagged for the identical
behaviour. It is not guaranteed to clear this exact behavioural rule on day one,
before the signed binary builds reputation, but it is the only real fix.

Code signing also addresses Windows Defender's "unknown publisher" SmartScreen
prompt and its generic ML detections (Bearfoos) on the unsigned builds.

## What stays true

- No published tag, release or asset is ever modified. Signing changes the bytes
  of the binaries, so it happens **before** each release is packaged and
  published, never by re-signing a shipped asset.
- Signing runs only in hosted CI, never against the owner's machine or data.
- The portable filename `Network_Toolkit.exe` and the update-feed semantics do
  not change.

## Azure setup the owner does (once)

Account already created: Artifact Signing account **networktoolkit-signing**,
resource group **azure-signup-rg**, region **East US**
(endpoint `https://eus.codesigning.azure.net`).

1. **Finish identity validation.** In the Artifact Signing account →
   *Identity validations*, the individual Public validation must reach
   **Completed** (Microsoft reviews it; it currently shows *Action Required /
   In Progress*). Signing cannot be set up until this is Completed.

2. **Create a certificate profile.** Artifact Signing account → *Objects* →
   *Certificate profiles* → **Create**.
   - Profile type: **Public Trust**.
   - Identity validation: the completed validation from step 1.
   - Name: `jpaull-codesign` (done; wired into the workflow).

3. **App registration for GitHub OIDC** (no client secret, no password stored).
   Microsoft Entra ID → *App registrations* → **New registration** (name e.g.
   `networktoolkit-github-signing`). Then on it:
   - *Certificates & secrets* → *Federated credentials* → **Add credential** →
     scenario **GitHub Actions deploying Azure resources**:
     - Organization: `TechSavage1989`, Repository: `network-toolkit`.
     - Entity type **Environment**, environment name `release`. This generates
       the subject `repo:TechSavage1989@44094233/network-toolkit@1432533692:environment:release`.
     - Use an **Environment** credential, not a Tag one: Entra matches the
       subject literally, so a `v*` tag subject (`...:ref:refs/tags/v*`) never
       matches a real tag like `v1.21.85`. The signing job runs in the GitHub
       `release` environment, so its OIDC subject is the environment one above
       and covers both `main`-push and `v*`-tag release runs.
   - Note the **Application (client) ID** and the **Directory (tenant) ID**.

   Also create the environment in GitHub: repo → Settings → Environments →
   **New environment** → name `release`. Leave it without required reviewers
   unless you want a manual approval gate on every release (that would pause the
   signing job until someone approves it).

4. **Give the app permission to sign.** On the **networktoolkit-signing**
   account → *Access control (IAM)* → **Add role assignment** → role
   **Trusted Signing Certificate Profile Signer** (also shown as *Code Signing
   Certificate Profile Signer*) → assign to the app registration from step 3.

5. **GitHub repository secrets** (Settings → Secrets and variables → Actions).
   None of these is a password; the OIDC login needs only the identifiers:
   - `AZURE_TENANT_ID` — the Directory (tenant) ID.
   - `AZURE_CLIENT_ID` — the Application (client) ID.
   - `AZURE_SUBSCRIPTION_ID` — the subscription that holds the account.
   The endpoint, account name and certificate-profile name are not secret and
   will live in the workflow as plain values (or repository *variables*).

The three secrets are the only remaining gate. Until they are set, a release run
fails at the Azure sign-in step rather than publishing anything unsigned. Once
they are set, the next beta is built signed and we test it on the Server 2022
box. The role assignment (step 4) and the two federated credentials (step 3)
also have to be in place, or the sign-in / signing step fails.

## Workflow change (how it will work)

Release today builds the three Windows programs on Ubuntu, packages the Portable
ZIP, Update ZIP and Setup, writes `SHA256SUMS`, then verifies the installer on a
Windows runner and publishes. Authenticode signing needs Windows, and every
signed byte must be in place before the checksums are written, so the signing
and packaging move to a Windows job:

1. **Build (Ubuntu, unchanged):** produce the three unsigned `.exe` files plus
   the Linux archives; upload them as a build artifact.
2. **Sign + package (Windows, new):**
   - `azure/login` with OIDC (`permissions: id-token: write`), using the three
     secrets. No secret is printed or persisted.
   - Sign `Network_Toolkit.exe`, `Network_Toolkit_TextEditor.exe` and
     `Network_Toolkit_Terminal.exe` with Azure Artifact Signing.
   - Package as now (`github_release.py package`), which builds the Portable ZIP,
     Update ZIP and Setup from the **signed** exes.
   - Sign `Setup.exe` after NSIS builds it, before the final `SHA256SUMS`.
   - Write `SHA256SUMS` over the signed assets and run the existing verify.
3. **Publish (unchanged):** download the signed assets and publish, with the
   same guards that never overwrite an existing release.

`github_release.py package` gains an optional `--sign <command>` hook: the
command is run on each exe before the archives are built and on `Setup.exe`
before the checksums, so ordinary (unsigned) local builds and tests are
unchanged and a stub signer exercises the path in CI. The signer itself is
Microsoft's `sign` tool (or `azure/trusted-signing-action`) pointed at the
account endpoint and certificate profile.

### Not covered by signing

- The Linux archives are not Authenticode-signed (not applicable).
- Signing lowers but does not guarantee removal of the specific SentinelOne
  behavioural rule until the signed build earns reputation. If it still fires,
  the separate-window admin shell (Windows' own signed `cmd.exe`/`powershell.exe`)
  remains the fallback that moves the elevated shell host off our program.
