"""Build a strictly allowlisted binary-only release. Never copy source trees."""
import argparse
from pathlib import Path
import zipfile
import github_release as release

def companion_member(current, name):
    """Update ZIP member of a separate program, e.g. Network_Toolkit_Terminal_1.21.45_Windows_x64.exe.

    It is not an official versioned name of the main program, which released
    updaters read (Network_Toolkit_<version>_Windows_x64.exe), and they ignore it.
    """
    return name[:-len('.exe')] + '_' + current + '_Windows_x64.exe'


def package(binary_path, output, write_manifest=True, programs=None):
    """The binary-only Update ZIP. programs: {separate program name: path}, both or none."""
    current = release.validate()
    binary = binary_path.read_bytes()
    release.check_exe(binary, current)
    root = 'Network_Toolkit_' + current
    name = root + '_Windows_x64.exe'
    entries = {name: binary}
    programs = programs or {}
    release.require(set(programs) in (set(), set(release.SEPARATE_PROGRAMS)), 'Provide both separate programs or neither')
    for program, path in programs.items():
        data = path.read_bytes()
        release.check_program(data, current, program)
        entries[companion_member(current, program)] = data
    for filename in ['UPDATING.txt', 'THIRD_PARTY_NOTICES.txt']:
        content = (release.ROOT / 'docs' / filename).read_text()
        if filename == 'THIRD_PARTY_NOTICES.txt':
            content = content.replace('NETWORK TOOLKIT 1.17.35', 'NETWORK TOOLKIT ' + current)
            content = content.replace('included official Go x/crypto and x/sys modules', 'official Go x/crypto and x/sys modules')
            content = content.replace('The loader adaptation is included as source and compiled into the EXE.', 'The loader adaptation is compiled into the EXE.')
            content = content.replace('Required source files and module licenses are included in source/vendor.', 'Required source files and module licenses are available in the official source repository.')
            content = content.replace('source/', 'the source repository: ')
        entries[filename] = content.encode()
    entries['SHA256SUMS'] = release.sums(entries)
    output.mkdir(parents=True, exist_ok=True)
    archive = output / (root + '_Update.zip')
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as z:
        for filename, content in sorted(entries.items()):
            z.writestr(root + '/' + filename, content)
    with zipfile.ZipFile(archive) as z:
        assert z.testzip() is None
        assert set(z.namelist()) == {root + '/' + n for n in entries}
        assert all(z.read(root + '/' + n) == data for n, data in entries.items())
    release.atomic_write(output / release.STABLE_EXE, binary)
    if write_manifest:
        release.atomic_write(output / 'SHA256SUMS', release.sums({archive.name: archive.read_bytes()}))
    print(archive)
    return archive

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--exe', type=Path, required=True)
    parser.add_argument('--output-dir', type=Path, default=Path('public-dist'))
    args = parser.parse_args()
    package(args.exe, args.output_dir)


def verify_archive(archive, binary, current, programs=None):
    """programs: {separate program name: bytes} that the archive must carry, or None for none."""
    root = 'Network_Toolkit_' + current + '/'
    executable = 'Network_Toolkit_' + current + '_Windows_x64.exe'
    expected = {executable, 'UPDATING.txt', 'THIRD_PARTY_NOTICES.txt', 'SHA256SUMS'}
    expected.update(companion_member(current, name) for name in programs or {})
    with zipfile.ZipFile(archive) as bundle:
        names = bundle.namelist()
        release.require(len(names) == len(expected) and set(names) == {root + n for n in expected},
                        'Update ZIP must contain exactly the six allowlisted members' if programs else
                        'Update ZIP must contain exactly the four allowlisted members')
        release.require(bundle.testzip() is None, 'Update ZIP CRC verification failed')
        checked = release.check_sums(bundle.read(root + 'SHA256SUMS'), lambda n: bundle.read(root + n))
        release.require(checked == expected - {'SHA256SUMS'}, 'Update ZIP checksum coverage differs')
        release.require(bundle.read(root + executable) == binary, 'Update and standalone executables differ')
        release.check_exe(binary, current)
        for name, data in (programs or {}).items():
            release.require(bundle.read(root + companion_member(current, name)) == data,
                            f'Update and portable {name} differ')
            release.check_program(data, current, name)
