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
        self.assertIn(str(Path(self.temporary.name) / f'Network_Toolkit_{self.current}_Update.zip'), command)

    def test_beta_channel_is_published_as_github_prerelease(self):
        with mock.patch.object(release, 'release_channel', return_value='beta'):
            command = self.publish_with()
        self.assertIn('--prerelease', command)
        self.assertEqual(command[command.index('--title') + 1], 'Network Toolkit ' + self.current + ' Beta')
        self.assertNotIn('--clobber', command)

    def test_stable_channel_never_uses_prerelease_flag(self):
        with mock.patch.object(release, 'release_channel', return_value='stable'):
            command = self.publish_with()
        self.assertNotIn('--prerelease', command)

    def test_publisher_refuses_existing_release(self):
        with self.assertRaisesRegex(ValueError, 'refusing to replace'):
            self.publish_with(existing_release={'id': 1})

    def test_publisher_refuses_tag_at_different_commit(self):
        with self.assertRaisesRegex(ValueError, 'Published tag differs'):
            self.publish_with(tag_commit=OTHER_COMMIT)


if __name__ == '__main__':
    unittest.main()


def fake_elf(machine, version):
    import struct
    header = bytearray(64)
    header[:6] = b'\x7fELF\x02\x01'
    struct.pack_into('<HH', header, 16, 2, machine)
    return bytes(header) + b'\x00' * 64 + version.encode() + b'\x00synthetic'


class LinuxArchives(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.output = Path(self.temporary.name)
        self.current = release.version()

    def build(self, arch='x86_64', machine=62):
        return release.package_linux(fake_elf(machine, self.current), self.current, arch, machine, self.output)

    def test_archive_round_trip_is_reproducible_and_verified(self):
        first = self.build().read_bytes()
        second = self.build().read_bytes()
        self.assertEqual(first, second, 'Linux archives must be reproducible')
        import tarfile
        with tarfile.open(self.output / release.linux_archive_name(self.current, 'x86_64')) as bundle:
            program = bundle.getmember(f'Network_Toolkit_{self.current}_Linux_x86_64/network-toolkit')
            self.assertEqual(program.mode & 0o777, 0o755)
            self.assertEqual((program.uid, program.gid), (0, 0))
            self.assertIn(f'Network_Toolkit_{self.current}_Linux_x86_64/README.txt', bundle.getnames())

    def test_wrong_architecture_or_version_is_rejected(self):
        with self.assertRaises(ValueError):
            release.package_linux(fake_elf(183, self.current), self.current, 'x86_64', 62, self.output)
        with self.assertRaises(ValueError):
            release.package_linux(fake_elf(62, '0.0.1'), self.current, 'x86_64', 62, self.output)
        with self.assertRaises(ValueError):
            release.package_linux(b'MZ' + b'\x00' * 200 + self.current.encode(), self.current, 'x86_64', 62, self.output)

    def rewrite(self, mutate):
        import gzip, tarfile
        archive = self.build()
        members = []
        with tarfile.open(archive) as bundle:
            for member in bundle.getmembers():
                data = bundle.extractfile(member).read() if member.isfile() else None
                members.append((member, data))
        mutate(members)
        raw = io.BytesIO()
        with gzip.GzipFile(fileobj=raw, mode='wb', mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode='w', format=tarfile.USTAR_FORMAT) as bundle:
                for member, data in members:
                    bundle.addfile(member, io.BytesIO(data) if data is not None else None)
        archive.write_bytes(raw.getvalue())
        return archive

    def test_unsafe_or_unlisted_entries_are_rejected(self):
        import tarfile
        prefix = f'Network_Toolkit_{self.current}_Linux_x86_64'
        def add_symlink(members):
            link = tarfile.TarInfo(prefix + '/escape')
            link.type, link.linkname = tarfile.SYMTYPE, '/etc/passwd'
            members.append((link, None))
        def add_extra(members):
            extra = tarfile.TarInfo(prefix + '/extra.sh')
            extra.size, extra.mode = 3, 0o644
            members.append((extra, b'x=1'))
        def make_writable(members):
            for member, _ in members:
                if member.name.endswith('/network-toolkit'):
                    member.mode = 0o777
        def escape(members):
            outside = tarfile.TarInfo('../outside')
            outside.size = 1
            members.append((outside, b'x'))
        for mutate in (add_symlink, add_extra, make_writable, escape):
            archive = self.rewrite(mutate)
            with self.subTest(mutate.__name__), self.assertRaises(ValueError):
                release.verify_linux(archive, self.current, 'x86_64', 62)

    def test_release_asset_order_keeps_checksums_last_and_windows_names(self):
        names = release.release_assets(self.current)
        self.assertEqual(names[:3], ['Network_Toolkit.exe', f'Network_Toolkit_{self.current}_Portable.zip', f'Network_Toolkit_{self.current}_Update.zip'])
        self.assertEqual(names[-1], 'SHA256SUMS')
        self.assertIn(f'Network_Toolkit_{self.current}_Linux_x86_64.tar.gz', names)
        self.assertIn(f'Network_Toolkit_{self.current}_Linux_aarch64.tar.gz', names)

    def test_windows_setup_is_required_only_starting_with_its_first_release(self):
        self.assertNotIn(release.windows_setup_name('1.21.34'), release.release_assets('1.21.34'))
        self.assertIn(release.windows_setup_name('1.21.35'), release.release_assets('1.21.35'))
        self.assertIn(release.windows_setup_name('1.22.0'), release.release_assets('1.22.0'))

    def test_windows_setup_verifies_the_nsis_file_and_product_versions(self):
        import struct
        data = bytearray(200)
        data[:2] = b'MZ'
        struct.pack_into('<I', data, 0x3c, 0x40)
        data[0x40:0x44] = b'PE\0\0'
        struct.pack_into('<H', data, 0x40 + 24 + 68, 2)
        version_words = release.windows_version_words('1.21.35')
        struct.pack_into('<6I', data, 160, 0xFEEF04BD, 0, *version_words, *version_words)
        release.check_windows_setup(data, '1.21.35')
        with self.assertRaisesRegex(ValueError, 'version resource'):
            release.check_windows_setup(data, '1.21.36')
