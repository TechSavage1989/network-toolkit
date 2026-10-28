# 1.17.109 candidate handoff

The user authorized implementation and publication on2026-10-28 at15:00:29UTC.
Scope is vertical workspace: remove redundant global header; relocate existing
identity/theme/settings/activity controls; compact editor Move access; independently
collapsible, remembered Local/Connected forms. Preserve108 behavior and security.

Baseline main00c290a78f354a31ae4638fc37ab85c7532a6fed / tree
a9fa5aa5424e270ef7053db6b4e827a23ae1ee04. Published108 source/tag
59a82b13bb31899206d383ea58166ec9a4f4faf0 and all four assets remain immutable.
Its duplicate main checks and docs-only closure were completed before109 source work.

Four private reference images were materialized and actually viewed. They show a
mostly empty global header, a separate Move-documents row and expanded local/remote
opening forms. They remain outside source/artifacts. Public tests use synthetic data.

The official Notepad++ manual informed compact command access and optional panels,
not a wholesale clone or an expansion of editor features:
- https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/views.md
- https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/user-interface.md
Toolkit retains its own acknowledged dirty-document transfers and capability model.

Acceptance requires actual before/after tool and editor geometry against exact108,
normal/narrow/light/dark/collapsed/resize cases, detached invariance, discoverable
keyboard controls and retained unsaved state, plus every existing required CI gate.
Authored tests or local source changes are not actual pixel/native acceptance.
Final hosted checks, package/source identity and public asset/feed verification
are pending. No user desktop, live devices, private documents or security settings
are used for development validation.
