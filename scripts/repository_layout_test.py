"""Protect the compact source layout when older feature branches are integrated."""
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


def misplaced_root_sources(root):
    allowed = {'resources.go', 'resources_test.go'}
    return sorted(path.name for path in root.iterdir()
                  if path.is_file() and ((path.suffix == '.go' and path.name not in allowed)
                                         or path.suffix == '.syso'))


class RepositoryLayoutTests(unittest.TestCase):
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
