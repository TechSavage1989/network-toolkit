# Network Toolkit GitHub setup

This folder contains the complete Network Toolkit 1.17.57 source, vendored dependencies, automated checks and release publishing. No password, GitHub token or signing key is included.

## Recommended setup through ChatGPT

1. In your GitHub account TechSavage1989, create a private repository named network-toolkit. Initializing it with a README is fine for the ChatGPT upload route.
2. Allow the ChatGPT GitHub connection to access this repository. If access is limited to selected repositories, add network-toolkit to that selection.
3. Return its repository URL to ChatGPT. ChatGPT can upload this prepared source and workflows through the connected GitHub account, subject to the connection's granted permissions.
4. The first push to main starts checks and release publishing. View the progress under the repository's Actions tab.

This route does not require Git, Python or GitHub CLI on your computer. Creating the repository and granting access are account actions that may require your participation.

## How later releases work

- Pushes and pull requests run the Checks workflow.
- A push to main reads the application version. If that version already has a release, it keeps the existing release and runs source checks separately.
- If the version is new, the Release workflow validates the version metadata, runs the application and JavaScript tests, embeds the Windows icon/version resources, builds the Windows x64 GUI executable and verifies the portable archive.
- Successful builds publish a new version tag and release at that exact source commit. Existing releases and tags pointing elsewhere are never replaced.
- Releases contain Network_Toolkit_Windows_x64.exe, a versioned Portable.zip with complete editable source, and SHA256SUMS.
- Version-tag pushes and manual Release workflow runs are also supported. For a manual run, select main and enter an existing version tag. Its commit must belong to main's history and match the source version.

ChatGPT can push future requested changes when the GitHub connection provides repository write access. GitHub Actions runs the building and publishing after the push. This is a repository workflow, not a continuously running ChatGPT background session.

Before publishing a new version, update the current version consistently in internal/model/model.go, web/index.html, build scripts, the Windows resource generator, README.txt and the current release notes. Regenerate resource_windows_amd64.syso by running Python on scripts/build_windows_resources.py. The validation helper rejects mismatches. Historical release notes can keep their original versions.

## Optional local setup on your Windows computer

Use this route only if you prefer to push from your own computer. Install Git, Python 3 and GitHub CLI first. The supplied scripts do not install software automatically.

- Git: https://git-scm.com/downloads
- Python: https://www.python.org/downloads/windows/
- GitHub CLI: https://cli.github.com/

Run Setup_GitHub.cmd from this folder. It authenticates through GitHub CLI's browser flow, verifies the TechSavage1989 account, creates a private network-toolkit repository if absent, creates the first local commit and pushes main. It stops if the repository already contains commits and never replaces existing history. Do not use this script after creating a README in GitHub; use the ChatGPT route or an existing clone instead.

For later local changes, work in the existing checkout and run Publish_Update.cmd. It validates the version, commits pending source changes and pushes main without forcing history. A new version is built and released by GitHub Actions. If main has newer remote commits, pull and resolve them before retrying. Publishing the same version does not replace its release.

GitHub CLI manages authentication. Initial sign-in requests the workflow permission needed to upload files under .github/workflows. If an existing GitHub CLI sign-in lacks this permission and GitHub rejects the push, run "gh auth refresh --hostname github.com --scopes workflow" on your own computer and approve it through GitHub. The script does not silently broaden an existing sign-in. The scripts use your verified GitHub account's no-reply commit email and configure the credential helper only in this repository. Do not paste a GitHub token into chat or application settings.

## Release verification

The release helper checks the GUI executable's AMD64 PE header and embedded version. ZIP publication uses a closed temporary archive, verifies all members and CRCs, then renames the completed archive into place. Separate source and release checksum manifests cover all package files.

Checks currently run the application's main package, internal/terminal and internal/desktop Go tests, all Node tests, and 13 release guard tests. The release guard tests cover exact source selection, version mismatches, branch restrictions, dirty source, existing releases, conflicting tags and authentication failures. Some broader network diagnostic tests depend on the host's kernel/network access and are not claimed as part of this CI gate. Browser rendering and live SSH/FTPS tests remain part of the development validation process.

The first GitHub workflow run must be checked after the repository is created. Preparing these files locally does not prove that the remote account's Actions permissions or runners have executed them.

## Application auto-updates

This setup prepares source hosting and automated releases. The application updater still needs to be added to Network Toolkit. A private repository requires authenticated update access; a separate public release repository is another option when downloads should work without a GitHub login. Update signatures and restart/rollback handling belong to that next implementation.
