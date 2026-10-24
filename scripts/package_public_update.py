"""Build a strictly allowlisted binary-only release. Never copy source trees."""
import argparse
from pathlib import Path
import zipfile
import github_release as release

def package(binary_path, output):
    current = release.validate()
    binary = binary_path.read_bytes()
    release.check_exe(binary, current)
    root = 'Network_Toolkit_' + current
    name = root + '_Windows_x64.exe'
    entries = {name: binary}
    for filename in ['UPDATING.txt', 'THIRD_PARTY_NOTICES.txt']:
        content = (release.ROOT / filename).read_text()
        if filename == 'THIRD_PARTY_NOTICES.txt':
            content = content.replace('NETWORK TOOLKIT 1.17.35', 'NETWORK TOOLKIT ' + current)
            content = content.replace('included official Go x/crypto and x/sys modules', 'official Go x/crypto and x/sys modules')
            content = content.replace('The loader adaptation is included as source and compiled into the EXE.', 'The loader adaptation is compiled into the EXE.')
            content = content.replace('Required source files and module licenses are included in source/vendor.', 'Required source files and module licenses are retained in the private source tree.')
            content = content.replace('source/', 'the private source tree: ')
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
    release.atomic_write(output / 'SHA256SUMS', release.sums({archive.name: archive.read_bytes()}))
    print(archive)
    return archive

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--exe', type=Path, required=True)
    parser.add_argument('--output-dir', type=Path, default=Path('public-dist'))
    args = parser.parse_args()
    package(args.exe, args.output_dir)
