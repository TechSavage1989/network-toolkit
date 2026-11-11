"""The separate Text Editor and Terminal programs (docs/SEPARATE_EXECUTABLES.md):
their Windows resources, release packaging, and Setup's script."""
from pathlib import Path
from unittest import mock
import runpy
import struct
import tempfile
import unittest
import zipfile

import github_release as release
import package_public_update

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = runpy.run_path(str(ROOT / 'scripts/build_windows_resources.py'))


def resource_section(obj):
    """The .rsrc section of a resource object written by build_windows_resources.py."""
    machine, sections = struct.unpack_from('<HH', obj)
    assert machine == 0x8664 and sections == 1
    size, raw = struct.unpack_from('<II', obj, 20 + 16)
    return obj[raw:raw + size]


def directory(section, at):
    """[(name or id, is_directory, offset)] in resource directory order."""
    named, numbered = struct.unpack_from('<HH', section, at + 12)
    entries = []
    for i in range(named + numbered):
        key, child = struct.unpack_from('<II', section, at + 16 + 8 * i)
        if key & 0x80000000:
            length, = struct.unpack_from('<H', section, key & 0x7FFFFFFF)
            start = (key & 0x7FFFFFFF) + 2
            key = section[start:start + 2 * length].decode('utf-16le')
        entries.append((key, bool(child & 0x80000000), child & 0x7FFFFFFF))
    return entries


def group_icons(obj):
    """Icon groups in the order Explorer counts icon indexes: {index: (key, group bytes)}."""
    section = resource_section(obj)
    types = dict((key, offset) for key, _, offset in directory(section, 0))
    groups = []
    for key, _, offset in directory(section, types[14]):
        language = directory(section, offset)[0][2]
        data_offset, size = struct.unpack_from('<II', section, language)
        groups.append((key, bytes(section[data_offset:data_offset + size])))
    return groups


def utf16(value):
    return (value + '\0').encode('utf-16le')


class ProgramResources(unittest.TestCase):
    def setUp(self):
        self.objects = {output: data for output, (data, _) in RESOURCES['program_resources']().items()}

    def test_committed_objects_are_current(self):
        for output, data in self.objects.items():
            self.assertEqual((ROOT / output).read_bytes(), data, f'Regenerate {output}')

    def test_main_program_icons_and_indexes_are_unchanged(self):
        main = self.objects['cmd/network-toolkit/resource_windows_amd64.syso']
        groups = group_icons(main)
        # Index 0 Toolkit, 1 Text Editor, 2 Terminal: what Setup's registrations
        # and the native window code (desktop.Icon*) name.
        self.assertEqual([key for key, _ in groups], [1, 2, 3])
        self.assertIn(utf16('Network_Toolkit.exe'), main)

    def test_each_separate_program_shows_its_own_icon_first_and_keeps_the_rest(self):
        main_groups = dict(group_icons(self.objects['cmd/network-toolkit/resource_windows_amd64.syso']))
        for output, app_group, original in (('internal/winres/editor/resource_windows_amd64.syso', 2, 'Network_Toolkit_TextEditor.exe'),
                                            ('internal/winres/terminal/resource_windows_amd64.syso', 3, 'Network_Toolkit_Terminal.exe')):
            groups = group_icons(self.objects[output])
            self.assertEqual([key for key, _ in groups], ['APP', 1, 2, 3], output)
            self.assertEqual(groups[0][1], main_groups[app_group], f'{output}: icon 0 is its own app icon')
            for key, data in groups[1:]:
                self.assertEqual(data, main_groups[key], f'{output}: group {key} matches the main program')

    def test_version_strings_name_each_program(self):
        names = {'Network_Toolkit.exe', 'Network_Toolkit_TextEditor.exe', 'Network_Toolkit_Terminal.exe'}
        descriptions = {'cmd/network-toolkit/resource_windows_amd64.syso': 'Network Toolkit - Network Diagnostics',
                        'internal/winres/editor/resource_windows_amd64.syso': 'Network Toolkit Text Editor',
                        'internal/winres/terminal/resource_windows_amd64.syso': 'Network Toolkit Terminal'}
        for output, description, original, _ in RESOURCES['PROGRAMS']:
            data = self.objects[output]
            self.assertEqual(description, descriptions[output])
            for value in ('Network Toolkit', 'NetworkToolkit', release.version(), release.version() + '.0', description, original):
                self.assertIn(utf16(value), data, f'{output}: {value}')
            for other in names - {original}:
                self.assertNotIn(utf16(other), data, f'{output} names {other}')

    def test_validate_checks_every_program(self):
        self.assertEqual(release.validate(), release.version())
        with tempfile.TemporaryDirectory() as temp:
            stale = Path(temp) / 'stale.syso'
            stale.write_bytes(b'stale')
            generator = dict(RESOURCES)
            programs = list(generator['PROGRAMS'])
            programs[2] = (str(stale), *programs[2][1:])
            generator['PROGRAMS'] = tuple(programs)
            real = runpy.run_path
            with mock.patch.object(release.runpy, 'run_path', side_effect=lambda path: generator if path.endswith('build_windows_resources.py') else real(path)):
                with self.assertRaisesRegex(ValueError, 'Regenerate'):
                    release.validate()


def fake_program(current, original):
    data = bytearray(256)
    data[:2] = b'MZ'
    struct.pack_into('<I', data, 0x3c, 0x40)
    data[0x40:0x44] = b'PE\0\0'
    struct.pack_into('<H', data, 0x44, 0x8664)
    struct.pack_into('<H', data, 0x40 + 24, 0x20b)
    struct.pack_into('<H', data, 0x40 + 24 + 68, 2)
    return bytes(data) + utf16(current + '.0') + utf16(original)


class SeparateProgramPackaging(unittest.TestCase):
    current = '1.21.45'

    def build(self, root, programs=True, swap=False):
        (root / 'docs').mkdir()
        for name in ['README.txt', f'RELEASE_NOTES_{self.current}.txt', 'VALIDATION.txt', 'UPDATING.txt', 'THIRD_PARTY_NOTICES.txt', 'LINUX_README.txt']:
            (root / name if name.startswith('RELEASE_NOTES_') else root / 'docs' / name).write_text('Public release instructions')
        source = root / 'resources.go'
        source.write_text('package networktoolkit')
        exe = root / 'Network_Toolkit.exe'
        exe.write_bytes(fake_program(self.current, 'Network_Toolkit.exe'))
        editor, terminal = root / 'editor.exe', root / 'terminal.exe'
        editor.write_bytes(fake_program(self.current, 'Network_Toolkit_TextEditor.exe'))
        terminal.write_bytes(fake_program(self.current, 'Network_Toolkit_Terminal.exe'))
        if swap:
            editor, terminal = terminal, editor
        linux = {}
        for arch, machine in release.LINUX_ARCHES:
            header = bytearray(64)
            header[:6] = b'\x7fELF\x02\x01'
            header[16:20] = (2).to_bytes(2, 'little') + machine.to_bytes(2, 'little')
            linux[arch] = root / ('network-toolkit-' + arch)
            linux[arch].write_bytes(bytes(header) + self.current.encode())
        self.setups = []

        def fake_setup(exe_path, current, output, given=None):
            self.setups.append(given)
            setup = output / release.windows_setup_name(current)
            setup.write_bytes(b'MZ synthetic setup')
            return setup
        self.patches = [mock.patch.object(release, 'ROOT', root), mock.patch.object(release, 'validate', return_value=self.current),
                        mock.patch.object(release, 'source_files', return_value=[source]),
                        mock.patch.object(release, 'package_windows_setup', side_effect=fake_setup),
                        mock.patch.object(release, 'check_windows_setup')]
        for patch in self.patches:
            patch.start()
            self.addCleanup(patch.stop)
        given = {release.EDITOR_EXE: editor, release.TERMINAL_EXE: terminal} if programs else None
        output = root / 'dist'
        release.package(exe, output, 'v' + self.current, False, linux, given)
        return output

    def test_portable_update_and_setup_carry_the_three_programs_and_assets_stay_seven(self):
        with tempfile.TemporaryDirectory() as temp:
            output = self.build(Path(temp))
            self.assertEqual(release.verify(output, 'v' + self.current), self.current)
            self.assertEqual(set(self.setups[0]), {release.EDITOR_EXE, release.TERMINAL_EXE}, 'Setup is built with both programs')
            names = release.check_sums((output / 'SHA256SUMS').read_bytes(), lambda n: (output / n).read_bytes())
            self.assertEqual(names | {'SHA256SUMS'}, set(release.release_assets(self.current)))
            self.assertEqual(len(release.release_assets(self.current)), 7, 'the separate programs are not release assets of their own')
            prefix = f'Network_Toolkit_{self.current}/'
            with zipfile.ZipFile(output / f'Network_Toolkit_{self.current}_Portable.zip') as portable:
                executables = {n[len(prefix):] for n in portable.namelist() if n.endswith('.exe')}
                self.assertEqual(executables, {'Network_Toolkit.exe', 'Network_Toolkit_TextEditor.exe', 'Network_Toolkit_Terminal.exe'})
                self.assertIn(b'Network_Toolkit_Terminal.exe"', portable.read(prefix + 'Open_Terminal.cmd'))
                editor = portable.read(prefix + 'Network_Toolkit_TextEditor.exe')
            with zipfile.ZipFile(output / f'Network_Toolkit_{self.current}_Update.zip') as update:
                members = {n[len(prefix):] for n in update.namelist()}
                self.assertEqual(members, {f'Network_Toolkit_{self.current}_Windows_x64.exe', f'Network_Toolkit_TextEditor_{self.current}_Windows_x64.exe',
                                           f'Network_Toolkit_Terminal_{self.current}_Windows_x64.exe', 'UPDATING.txt', 'THIRD_PARTY_NOTICES.txt', 'SHA256SUMS'})
                inner = release.check_sums(update.read(prefix + 'SHA256SUMS'), lambda n: update.read(prefix + n))
                self.assertEqual(inner, members - {'SHA256SUMS'}, 'the inner SHA256SUMS covers every program')
                self.assertEqual(update.read(prefix + f'Network_Toolkit_TextEditor_{self.current}_Windows_x64.exe'), editor)
                # The member released updaters read keeps its name and bytes.
                self.assertEqual(update.read(prefix + f'Network_Toolkit_{self.current}_Windows_x64.exe'), (output / 'Network_Toolkit.exe').read_bytes())

    def test_verify_refuses_an_update_zip_without_the_separate_programs(self):
        with tempfile.TemporaryDirectory() as temp:
            output = self.build(Path(temp))
            archive = output / f'Network_Toolkit_{self.current}_Update.zip'
            package_public_update.package(output / 'Network_Toolkit.exe', output, write_manifest=False)
            release.atomic_write(output / 'SHA256SUMS', release.sums({n: (output / n).read_bytes() for n in release.release_assets(self.current)[:-1]}))
            with self.assertRaisesRegex(ValueError, 'six allowlisted'):
                release.verify(output, 'v' + self.current)
            self.assertTrue(archive.is_file())

    def test_swapped_or_missing_programs_are_refused(self):
        with tempfile.TemporaryDirectory() as temp, self.assertRaisesRegex(ValueError, 'version resource'):
            self.build(Path(temp), swap=True)
        with tempfile.TemporaryDirectory() as temp, self.assertRaisesRegex(ValueError, 'separate Text Editor and Terminal programs'):
            self.build(Path(temp), programs=False)

    def test_check_program_needs_its_own_name_and_no_other(self):
        release.check_program(fake_program(self.current, 'Network_Toolkit_Terminal.exe'), self.current, 'Network_Toolkit_Terminal.exe')
        with self.assertRaisesRegex(ValueError, 'own version resource'):
            release.check_program(fake_program(self.current, 'Network_Toolkit.exe'), self.current, 'Network_Toolkit_Terminal.exe')
        with self.assertRaisesRegex(ValueError, 'another program'):
            release.check_program(fake_program(self.current, 'Network_Toolkit_Terminal.exe') + utf16('Network_Toolkit.exe'), self.current, 'Network_Toolkit_Terminal.exe')
        with self.assertRaisesRegex(ValueError, 'version resource'):
            release.check_program(fake_program('1.21.44', 'Network_Toolkit_Terminal.exe'), self.current, 'Network_Toolkit_Terminal.exe')

    def test_versions_before_the_gate_keep_the_earlier_packages(self):
        self.assertTrue(release.has_separate_programs('1.21.44'))
        self.assertTrue(release.has_separate_programs('1.22.0'))
        self.assertFalse(release.has_separate_programs('1.21.43'))
        self.assertFalse(release.has_separate_programs('1.17.63'))


class SetupScript(unittest.TestCase):
    def setUp(self):
        self.nsi = (ROOT / 'scripts/installer/windows.nsi').read_text()

    def test_shortcuts_start_the_separate_programs_and_registrations_keep_their_commands(self):
        self.assertIn('!insertmacro CreateStartShortcut "$SMPROGRAMS\\${APP_NAME}\\${EDITOR_NAME}.lnk" "${EDITOR_FILENAME}" "" "${EDITOR_ARGS}" "--editor" 0', self.nsi)
        self.assertIn('!insertmacro CreateStartShortcut "$SMPROGRAMS\\${APP_NAME}\\${APP_NAME} Terminal.lnk" "${TERMINAL_FILENAME}" "" "--terminal" "--terminal" 0', self.nsi)
        self.assertIn('!define EDITOR_COMMAND \'"$INSTDIR\\${APP_FILENAME}" ${EDITOR_ARGS} --open-file "%1"\'', self.nsi)
        self.assertIn('\'"$INSTDIR\\${APP_FILENAME}" --terminal --open-url "%1"\'', self.nsi)
        # Registered icons still point at the main program's indexes.
        self.assertIn('\'"$INSTDIR\\${APP_FILENAME}",1\'', self.nsi)
        self.assertIn('\'"$INSTDIR\\${APP_FILENAME}",2\'', self.nsi)

    def test_every_program_is_installed_checked_and_removed(self):
        for name in ('APP_FILENAME', 'EDITOR_FILENAME', 'TERMINAL_FILENAME'):
            self.assertIn(f'!insertmacro IsProgramOpen "$INSTDIR\\${{{name}}}"', self.nsi)
            self.assertIn(f'!insertmacro RemoveProgram "${{{name}}}"', self.nsi)
        self.assertIn('File /oname=$StagedEditor "${EDITOR_EXE}"', self.nsi)
        self.assertIn('File /oname=$StagedTerminal "${TERMINAL_EXE}"', self.nsi)
        # The separate programs are placed before Network_Toolkit.exe.
        order = [self.nsi.index(f'!insertmacro PlaceStagedFile "{staged}"') for staged in ('$StagedEditor', '$StagedTerminal', '$StagedApp')]
        self.assertEqual(order, sorted(order))
        self.assertLess(self.nsi.index('!insertmacro AreProgramsOpen'), order[0], 'nothing is placed before every program is checked')
        # Each refusal and step has a reason in the log.
        self.assertIn('StrCpy $FailedStep "placing the Text Editor program file (Windows error $1)"', self.nsi)
        self.assertIn('StrCpy $FailedStep "placing the Terminal program file (Windows error $1)"', self.nsi)


if __name__ == '__main__':
    unittest.main()
