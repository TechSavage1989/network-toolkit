"""Read-only mocks for explicit same-version publication safeguards."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import github_release as release
import revise_release as revision

OLD = 'a' * 40
NEW = 'b' * 40
MANIFEST = 'c' * 64
REPO = 'TechSavage1989/network-toolkit'
TAG = 'v1.17.63'


def original():
    return {'id': 10, 'draft': False, 'prerelease': False, 'body': 'original', 'assets': [
        {'id': 20, 'name': 'Network_Toolkit.exe', 'size': 10, 'state': 'uploaded', 'digest': 'sha256:' + 'd' * 64},
        {'id': 21, 'name': 'SHA256SUMS', 'size': 10, 'state': 'uploaded', 'digest': 'sha256:' + MANIFEST}]}


class RevisionGuards(unittest.TestCase):
    def snapshot(self, commit=OLD, item=None):
        with mock.patch.object(release, 'gh_json', side_effect=[{'object': {'type': 'commit', 'sha': commit}}, item or original()]):
            return revision.snapshot(REPO, TAG, OLD, MANIFEST)

    def test_exact_original_snapshot(self):
        state = self.snapshot()
        self.assertEqual(state['release_id'], 10)
        self.assertEqual(state['old_commit'], OLD)

    def test_tag_movement_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'tag changed'):
            self.snapshot(commit=NEW)

    def test_changed_manifest_is_rejected(self):
        item = original()
        item['assets'][1]['digest'] = 'sha256:' + 'e' * 64
        with self.assertRaisesRegex(ValueError, 'manifest changed'):
            self.snapshot(item=item)

    def test_unknown_assets_are_not_deleted(self):
        item = original()
        item['assets'].append({'name': 'extra-user-file.txt'})
        with self.assertRaisesRegex(ValueError, 'inventory'):
            self.snapshot(item=item)

    def test_missing_original_digest_is_rejected(self):
        item = original()
        item['assets'][0]['digest'] = None
        with self.assertRaisesRegex(ValueError, 'digest'):
            self.snapshot(item=item)

    def test_revision_requires_explicit_manual_event(self):
        with mock.patch.object(release, 'git', return_value=''), mock.patch.dict(release.os.environ, {'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/main'}):
            with self.assertRaisesRegex(ValueError, 'explicit manual'):
                release.prepare(TAG, 'main', True, OLD, MANIFEST)

    def test_request_branch_is_bounded(self):
        with mock.patch.dict(release.os.environ, {'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/main'}):
            with self.assertRaisesRegex(ValueError, 'exact branch'):
                release.prepare_request('not-read.json', 'main')

    def test_request_cannot_target_another_repository(self):
        with mock.patch.dict(release.os.environ, {'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/release-revision-v1.17.63', 'GITHUB_REPOSITORY': 'other/repository'}):
            with self.assertRaisesRegex(ValueError, 'main repository'):
                release.prepare_request('not-read.json', 'main')

    def test_request_selects_exact_source_and_records_request_commit(self):
        self.check_request('.github/revision-request.json', False)

    def test_request_rejects_extra_code_changes(self):
        self.check_request('.github/revision-request.json\nupdater.go', True)

    def test_request_rejects_stale_source(self):
        self.check_request('.github/revision-request.json', True, stale=True)

    def test_request_rejects_altered_original(self):
        self.check_request('.github/revision-request.json', True, altered=True)

    def check_request(self, changed, reject, stale=False, altered=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            request = root / 'request.json'
            request.write_text(json.dumps({'tag': TAG,
                'old_commit': OLD if altered else '8df9b401111e09767591491bc973b4160a17a9bc',
                'old_manifest': '513dbd928837fe23785cf89a0bcbdb3d94613d7b759efcd0884459fc4116225f',
                'new_commit': NEW}))
            request_commit = 'f' * 40
            env = {'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/release-revision-v1.17.63',
                   'GITHUB_REPOSITORY': REPO, 'GITHUB_SHA': request_commit, 'GITHUB_OUTPUT': str(root / 'out')}
            with mock.patch.dict(release.os.environ, env), mock.patch.object(release, 'git', side_effect=[request_commit, changed]), mock.patch.object(release, 'gh_json', return_value={'object': {'sha': OLD if stale else NEW}}), mock.patch.object(release.subprocess, 'run') as run, mock.patch.object(release, 'prepare') as prepare:
                if reject:
                    with self.assertRaisesRegex(ValueError, 'authorized original' if altered else 'no longer main' if stale else 'only its request file'):
                        release.prepare_request(request, 'main')
                    prepare.assert_not_called()
                else:
                    release.prepare_request(request, 'main')
                    self.assertIn('request_commit=' + request_commit, (root / 'out').read_text())
                    self.assertTrue(prepare.call_args.kwargs['request_branch'])
                    self.assertEqual(run.call_args.args[0], ['git', 'checkout', '--detach', NEW])

    def test_changed_backup_blocks_all_mutation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            state = self.snapshot()
            (root / 'release-before.json').write_text(json.dumps(state))
            (root / 'Network_Toolkit.exe').write_bytes(b'changed')
            with mock.patch.object(release, 'verify', return_value='1.17.63'), mock.patch.object(revision, 'snapshot', return_value=state), mock.patch.object(revision.subprocess, 'run') as run:
                with self.assertRaisesRegex(ValueError, 'backup is missing or changed'):
                    revision.publish(root, root, REPO, TAG, OLD, MANIFEST, NEW)
                run.assert_not_called()

    def test_changed_release_blocks_all_mutation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            state = self.snapshot()
            (root / 'release-before.json').write_text(json.dumps(state))
            with mock.patch.object(release, 'verify', return_value='1.17.63'), mock.patch.object(revision, 'snapshot', return_value={}), mock.patch.object(revision.subprocess, 'run') as run:
                with self.assertRaisesRegex(ValueError, 'changed after backup'):
                    revision.publish(root, root, REPO, TAG, OLD, MANIFEST, NEW)
                run.assert_not_called()

    def test_partial_upload_failure_stops_before_manifest_and_tag(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            backup = root / 'backup'
            backup.mkdir()
            old_bytes = b'old original'
            state = {'assets': [{'name': 'Network_Toolkit.exe', 'size': len(old_bytes),
                                 'digest': 'sha256:' + release.sha(old_bytes)}]}
            (backup / 'Network_Toolkit.exe').write_bytes(old_bytes)
            (backup / 'release-before.json').write_text(json.dumps(state))
            import subprocess
            with mock.patch.object(release, 'verify', return_value='1.17.63'), mock.patch.object(revision, 'snapshot', return_value=state), mock.patch.object(release, 'git', return_value=NEW), mock.patch.object(release, 'gh_json', return_value={'object': {'sha': NEW}}), mock.patch.object(revision.subprocess, 'run', side_effect=[None, subprocess.CalledProcessError(1, 'upload')]) as run:
                with self.assertRaises(subprocess.CalledProcessError):
                    revision.publish(root, backup, REPO, TAG, OLD, MANIFEST, NEW)
                calls = [call.args[0] for call in run.call_args_list]
                self.assertEqual(len(calls), 2)
                self.assertIn(str(root / 'Network_Toolkit.exe'), calls[0])
                self.assertIn(str(root / 'Network_Toolkit_1.17.63_Portable.zip'), calls[1])
                self.assertFalse(any('api' in call or 'edit' in call for call in calls))

if __name__ == '__main__':
    unittest.main()
