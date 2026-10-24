"""Release safety checks without network access or account changes."""
import contextlib
import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import github_release as release


COMMIT = 'a' * 40
OTHER_COMMIT = 'b' * 40
REPOSITORY = 'TechSavage1989/network-toolkit'


class ReleaseGuards(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.output = Path(self.temporary.name) / 'output'
        self.current = release.version()
        self.tag = 'v' + self.current
        environment = {'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/main',
                       'GITHUB_REPOSITORY': REPOSITORY, 'GITHUB_OUTPUT': str(self.output)}
        self.environment = mock.patch.dict(os.environ, environment)
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.git = mock.patch.object(release, 'git', side_effect=lambda *args: '' if args[0] == 'status' else COMMIT)
        self.git_mock = self.git.start()
        self.addCleanup(self.git.stop)
        self.silent = contextlib.redirect_stdout(io.StringIO())
        self.silent.__enter__()
        self.addCleanup(self.silent.__exit__, None, None, None)

    def read_output(self):
        return dict(line.split('=', 1) for line in self.output.read_text().splitlines())

    def test_new_main_version_selects_exact_commit(self):
        with mock.patch.object(release, 'gh_json', return_value=None), mock.patch.object(release, 'remote_tag', return_value=None):
            release.prepare('', 'main')
        self.assertEqual(self.read_output(), {'publish': 'true', 'tag': self.tag, 'version': self.current, 'commit': COMMIT})

    def test_existing_release_is_skipped(self):
        with mock.patch.object(release, 'gh_json', return_value={'id': 1}), mock.patch.object(release, 'remote_tag') as lookup:
            release.prepare('', 'main')
        self.assertEqual(self.read_output()['publish'], 'false')
        lookup.assert_not_called()

    def test_existing_tag_at_another_commit_stops_publication(self):
        with mock.patch.object(release, 'gh_json', return_value=None), mock.patch.object(release, 'remote_tag', return_value=OTHER_COMMIT):
            with self.assertRaisesRegex(ValueError, 'another source commit'):
                release.prepare('', 'main')
        self.assertFalse(self.output.exists())

    def test_dirty_checkout_is_rejected(self):
        self.git_mock.side_effect = None
        self.git_mock.return_value = ' M web/index.html'
        with self.assertRaisesRegex(ValueError, 'uncommitted'):
            release.prepare('', 'main')

    def test_nondefault_branch_push_is_not_a_release(self):
        with mock.patch.dict(os.environ, {'GITHUB_REF': 'refs/heads/feature'}):
            with self.assertRaises(ValueError):
                release.prepare('', 'main')

    def test_tag_event_must_match_requested_tag(self):
        with mock.patch.dict(os.environ, {'GITHUB_REF': 'refs/tags/v0.0.1'}):
            with self.assertRaisesRegex(ValueError, 'Push ref differs'):
                release.prepare(self.tag, 'main')

    def test_manual_run_requires_default_branch(self):
        with mock.patch.dict(os.environ, {'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REF': 'refs/heads/feature'}):
            with self.assertRaisesRegex(ValueError, 'default branch'):
                release.prepare(self.tag, 'main')

    def test_manual_tag_checks_ancestry_and_exact_checkout(self):
        with mock.patch.dict(os.environ, {'GITHUB_EVENT_NAME': 'workflow_dispatch'}), mock.patch.object(release.subprocess, 'run') as run:
            release.prepare(self.tag, 'main')
        self.assertEqual(run.call_args_list[0].args[0], ['git', 'merge-base', '--is-ancestor', COMMIT, 'origin/main'])
        self.assertEqual(run.call_args_list[1].args[0], ['git', 'checkout', '--detach', 'refs/tags/' + self.tag])
        self.assertEqual(self.read_output()['commit'], COMMIT)

    def test_metadata_rejects_wrong_release_version(self):
        with self.assertRaisesRegex(ValueError, 'must match application version'):
            release.validate('v0.0.1')

    def test_only_missing_release_responses_are_ignored(self):
        denied = subprocess.CompletedProcess([], 1, '', 'gh: Bad credentials (HTTP 401)')
        with mock.patch.object(release.subprocess, 'run', return_value=denied):
            with self.assertRaisesRegex(ValueError, 'GitHub request failed'):
                release.gh_json('repos/example/project/releases/latest', missing=True)
        missing = subprocess.CompletedProcess([], 1, '', 'gh: Not Found (HTTP 404)')
        with mock.patch.object(release.subprocess, 'run', return_value=missing):
            self.assertIsNone(release.gh_json('repos/example/project/releases/latest', missing=True))

    def publish_with(self, existing_release=None, tag_commit=None):
        with mock.patch.object(release, 'verify', return_value=self.current), mock.patch.object(release, 'remote_tag', return_value=tag_commit), mock.patch.object(release, 'gh_json', return_value=existing_release), mock.patch.object(release.subprocess, 'run') as create:
            release.publish(Path(self.temporary.name), REPOSITORY, self.tag)
            return create.call_args.args[0]

    def test_new_release_targets_validated_commit(self):
        command = self.publish_with()
        self.assertEqual(command[:4], ['gh', 'release', 'create', self.tag])
        self.assertEqual(command[command.index('--target') + 1], COMMIT)
        self.assertEqual(command[command.index('--repo') + 1], REPOSITORY)
        self.assertNotIn('--clobber', command)

    def test_publisher_refuses_existing_release(self):
        with self.assertRaisesRegex(ValueError, 'refusing to replace'):
            self.publish_with(existing_release={'id': 1})

    def test_publisher_refuses_tag_at_different_commit(self):
        with self.assertRaisesRegex(ValueError, 'Published tag differs'):
            self.publish_with(tag_commit=OTHER_COMMIT)


if __name__ == '__main__':
    unittest.main()
