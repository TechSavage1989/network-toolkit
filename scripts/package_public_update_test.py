import tempfile
import unittest
from pathlib import Path
from unittest import mock
import zipfile
import package_public_update as package

class PublicPackageTests(unittest.TestCase):
    def test_strict_allowlist_never_copies_private_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); (root/'private.go').write_text('DO NOT PUBLISH')
            (root/'UPDATING.txt').write_text('Instructions')
            (root/'THIRD_PARTY_NOTICES.txt').write_text('Notices')
            exe=root/'test.exe';exe.write_bytes(b'MZbinary')
            with mock.patch.object(package.release,'ROOT',root),mock.patch.object(package.release,'validate',return_value='1.17.58'),mock.patch.object(package.release,'check_exe'):
                archive=package.package(exe,root/'output')
            with zipfile.ZipFile(archive) as z:
                self.assertEqual(set(z.namelist()),{'Network_Toolkit_1.17.58/'+x for x in ['Network_Toolkit_1.17.58_Windows_x64.exe','UPDATING.txt','THIRD_PARTY_NOTICES.txt','SHA256SUMS']})
                self.assertFalse(any(b'DO NOT PUBLISH' in z.read(n) for n in z.namelist()))
            self.assertEqual((root/'output/SHA256SUMS').read_text(),package.release.sha(archive.read_bytes())+'  '+archive.name+'\n')

if __name__=='__main__':unittest.main()
