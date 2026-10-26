import tempfile
import unittest
from pathlib import Path
from unittest import mock
import zipfile
import package_public_update as package

class PublicPackageTests(unittest.TestCase):
    def test_strict_allowlist_never_copies_private_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); (root/'docs').mkdir(); (root/'private.go').write_text('DO NOT PUBLISH')
            (root/'docs/UPDATING.txt').write_text('Instructions')
            (root/'docs/THIRD_PARTY_NOTICES.txt').write_text('Notices')
            exe=root/'test.exe';exe.write_bytes(b'MZbinary')
            with mock.patch.object(package.release,'ROOT',root),mock.patch.object(package.release,'validate',return_value='1.17.58'),mock.patch.object(package.release,'check_exe'):
                archive=package.package(exe,root/'output')
            with zipfile.ZipFile(archive) as z:
                self.assertEqual(set(z.namelist()),{'Network_Toolkit_1.17.58/'+x for x in ['Network_Toolkit_1.17.58_Windows_x64.exe','UPDATING.txt','THIRD_PARTY_NOTICES.txt','SHA256SUMS']})
                self.assertFalse(any(b'DO NOT PUBLISH' in z.read(n) for n in z.namelist()))
            self.assertEqual((root/'output/SHA256SUMS').read_text(),package.release.sha(archive.read_bytes())+'  '+archive.name+'\n')

class CombinedPackageTests(unittest.TestCase):
    def test_combined_release_and_strict_update_verification(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'docs').mkdir()
            for name in ['README.txt', 'RELEASE_NOTES_1.17.63.txt', 'VALIDATION.txt', 'UPDATING.txt', 'THIRD_PARTY_NOTICES.txt']:
                (root / name if name.startswith('RELEASE_NOTES_') else root / 'docs' / name).write_text('Public release instructions')
            source = root / 'cmd/network-toolkit/main.go'
            source.parent.mkdir(parents=True)
            source.write_text('package main')
            resources = root / 'resources.go'
            resources.write_text('package networktoolkit')
            exe = root / 'test.exe'
            exe.write_bytes(b'MZbinary')
            output = root / 'dist'
            with mock.patch.object(package.release, 'ROOT', root), mock.patch.object(package.release, 'validate', return_value='1.17.63'), mock.patch.object(package.release, 'check_exe'), mock.patch.object(package.release, 'source_files', return_value=[source, resources]):
                package.release.package(exe, output, 'v1.17.63', False)
                self.assertEqual(package.release.verify(output, 'v1.17.63'), '1.17.63')
                with zipfile.ZipFile(output / 'Network_Toolkit_1.17.63_Portable.zip') as portable:
                    prefix = 'Network_Toolkit_1.17.63/'
                    self.assertEqual(portable.read(prefix + 'source/cmd/network-toolkit/main.go'), source.read_bytes())
                    self.assertEqual(portable.read(prefix + 'source/resources.go'), resources.read_bytes())
                    self.assertEqual(portable.read(prefix + 'README.txt'), (root / 'docs/README.txt').read_bytes())
                    self.assertEqual(portable.read(prefix + 'VALIDATION.txt'), (root / 'docs/VALIDATION.txt').read_bytes())
                names = package.release.check_sums((output / 'SHA256SUMS').read_bytes(), lambda n: (output / n).read_bytes())
                self.assertEqual(names, {'Network_Toolkit.exe', 'Network_Toolkit_1.17.63_Portable.zip', 'Network_Toolkit_1.17.63_Update.zip'})
                archive = output / 'Network_Toolkit_1.17.63_Update.zip'
                with zipfile.ZipFile(archive, 'a') as z:
                    z.writestr('Network_Toolkit_1.17.63/unexpected.go', 'not allowed')
                with self.assertRaisesRegex(ValueError, 'four allowlisted'):
                    package.verify_archive(archive, exe.read_bytes(), '1.17.63')

    def test_embedded_binary_must_match_standalone(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'docs').mkdir()
            for name in ['UPDATING.txt', 'THIRD_PARTY_NOTICES.txt']:
                (root / name if name.startswith('RELEASE_NOTES_') else root / 'docs' / name).write_text('Instructions')
            exe = root / 'test.exe'
            exe.write_bytes(b'MZbinary')
            with mock.patch.object(package.release, 'ROOT', root), mock.patch.object(package.release, 'validate', return_value='1.17.63'), mock.patch.object(package.release, 'check_exe'):
                archive = package.package(exe, root / 'output')
                with self.assertRaisesRegex(ValueError, 'executables differ'):
                    package.verify_archive(archive, b'MZdifferent', '1.17.63')

if __name__=='__main__':unittest.main()
