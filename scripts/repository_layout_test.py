"""Protect the compact source layout when older feature branches are integrated."""
from pathlib import Path
import tempfile
import re
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[1]


def misplaced_root_sources(root):
    allowed = {'resources.go', 'resources_test.go'}
    return sorted(path.name for path in root.iterdir()
                  if path.is_file() and ((path.suffix == '.go' and path.name not in allowed)
                                         or path.suffix == '.syso'))


ALLOWED_ROOT = {'go.mod', 'assets', 'BUILD_WINDOWS.cmd', 'Setup_GitHub.cmd', '.github', 'Open_Terminal.cmd', 'cmd', 'vendor', 'go.sum', 'AGENTS.md', 'package.json', 'resources.go', 'Publish_Update.cmd', 'README.md', 'build.sh', 'web', 'internal', 'scripts', 'themes', '.gitignore', 'docs'}


def unexpected_root(paths, version):
    allowed = ALLOWED_ROOT | {f'RELEASE_NOTES_{version}.txt'}
    return sorted({p.split('/')[0] for p in paths if p} - allowed)


class RepositoryLayoutTests(unittest.TestCase):
    def test_compact_tracked_root(self):
        version = re.search(r'const Version = "([0-9.]+)"',
                            (ROOT / 'internal/model/model.go').read_text()).group(1)
        if (ROOT / '.git').exists():
            paths = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT).decode().split('\0')
        else:
            paths = [p.name for p in ROOT.iterdir()
                     if p.name not in {'build', 'dist', 'node_modules', 'test-results'}]
        self.assertEqual(unexpected_root(paths, version), [], 'Group new guides, scripts and source; read AGENTS.md')

    def test_guard_rejects_future_root_clutter(self):
        self.assertEqual(unexpected_root(['README.md', 'scratch.txt', 'notes/new.md',
                                         'RELEASE_NOTES_0.0.1.txt'], '1.2.3'),
                         ['RELEASE_NOTES_0.0.1.txt', 'notes', 'scratch.txt'])

    def test_application_sources_stay_in_command_directory(self):
        self.assertEqual(misplaced_root_sources(ROOT), [],
                         'Move application Go files/tests/resources to cmd/network-toolkit; '
                         'read docs/DEVELOPMENT_HANDOFF.md before merging an older branch')
        for name in ['main.go', 'main_test.go', 'resource_windows_amd64.syso']:
            self.assertTrue((ROOT / 'cmd/network-toolkit' / name).is_file(), name)
        self.assertTrue((ROOT / 'resources.go').is_file())

    def test_guard_rejects_reintroduced_application_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ['resources.go', 'resources_test.go', 'new_feature.go',
                         'new_feature_test.go', 'resource_windows_amd64.syso']:
                (root / name).write_text('fixture')
            self.assertEqual(misplaced_root_sources(root),
                             ['new_feature.go', 'new_feature_test.go', 'resource_windows_amd64.syso'])


if __name__ == '__main__':
    unittest.main()
