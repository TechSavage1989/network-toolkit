# Network Toolkit release notes standard

Use this format for every future numbered release. The in-app update dialog
displays the published GitHub release body through
"web/assets/app-update-notes.js". It is not a developer changelog.

## Required sections

1. **What's New** is the short, user-facing summary. Write one to four brief
   bullets about the effect of the update on everyday use. Lead with what the
   user can do, what works better, or what problem has been fixed. Use ordinary
   language, name controls exactly as they appear in the app, and keep each
   bullet well under 180 characters so the preview is not truncated. Do not
   include implementation details, source filenames, internal components,
   commit hashes, test counts, build mechanics, or speculative improvements.

2. **Additional Details** is the expandable technical section. Explain the
   specific behavior, affected connection types and workflows, eligibility
   and scope, safeguards, constraints, interoperability, and important
   compatibility details. Include technical terms where they help; keep the
   text precise and readable. Explain known limitations honestly. If speed or
   device behavior has not been measured or verified, do not claim it has.

3. Use the exact Markdown section headings "## What's New" and
   "## Additional Details" in the root release note. The application uses the
   first section for the visible summary and the second for the optional
   disclosure. The release title can precede them. Do not mix technical bullets
   into the first section. Older unstructured releases remain supported by
   the updater's legacy notes parser and must not be rewritten.

## Example release note

    Network Toolkit 1.17.X

    ## What's New
    - Reconnect individual terminals or multiple sessions more easily.
    - Clearer update notes show the changes that matter first.

    ## Additional Details
    - Reconnect all includes eligible sessions owned by the current window
      and skips sessions that are unavailable or owned by detached windows.
    - A confirmation warns when active connections will be interrupted.
    - The updater retains existing packages, settings and shortcut behavior.

## Publication checklist

- Review the in-app "What's New" preview and expanded "Additional details" at
  normal and narrow window widths. Only user-facing benefit bullets should be
  in the preview; details should remain in the disclosure.
- Preserve accurate root release notes for the new version and archive the
  previous root note in "docs/releases" byte-for-byte.
- Keep more extensive test output, source identifiers, metrics, failures and
  release verification in the development/release handoff, not in the preview.
- Never include device names, addresses, credentials, private logs or copied
  customer data in published release notes.
- Keep published release assets, tags and notes immutable unless an explicitly
  authorized revision is being performed. Documentation-only edits do not
  implicitly authorize another executable release.
