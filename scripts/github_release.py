"""Validate, package and publish a Network Toolkit release using standard tools.

GitHub authentication stays in GitHub CLI or the workflow's temporary token.
No personal credentials are read from, written to, or included in the package.
"""
from pathlib import Path
import argparse
import hashlib
import json
import os
import re
import runpy
import struct
import subprocess
import sys
import tarfile
import gzip
import io
import zipfile

ROOT = Path(__file__).resolve().parents[1]
VERSION_PATTERN = r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)'
STABLE_EXE = 'Network_Toolkit.exe'
FIRST_WINDOWS_SETUP = (1, 21, 35)
# The separate Text Editor and Terminal programs (docs/SEPARATE_EXECUTABLES.md)
# ship inside the portable ZIP, the Update ZIP and Setup, never as assets of
# their own. v1.21.44 was published before them; the gate includes it so that
# release rehearsals of a checkout still at 1.21.44 build and check them.
# Published releases are never packaged again.
FIRST_SEPARATE_PROGRAMS = (1, 21, 44)
EDITOR_EXE = 'Network_Toolkit_TextEditor.exe'
TERMINAL_EXE = 'Network_Toolkit_Terminal.exe'
SEPARATE_PROGRAMS = (EDITOR_EXE, TERMINAL_EXE)
# Linux edition (docs/LINUX_PLAN.md): one portable .tar.gz per architecture.
LINUX_EXE = 'network-toolkit'
LINUX_ARCHES = (('x86_64', 62), ('aarch64', 183))  # release name, ELF e_machine
LINUX_MTIME = 1767225600  # 2026-01-01T00:00:00Z: reproducible archive entries


def linux_archive_name(current, arch):
    return f'Network_Toolkit_{current}_Linux_{arch}.tar.gz'


def windows_setup_name(current):
    return f'Network_Toolkit_{current}_Windows_x64_Setup.exe'


def has_windows_setup(current):
    return tuple(map(int, current.split('.'))) >= FIRST_WINDOWS_SETUP


def has_separate_programs(current):
    return tuple(map(int, current.split('.'))) >= FIRST_SEPARATE_PROGRAMS



def release_assets(current):
    """Every published asset, in upload order; SHA256SUMS is always last."""
    return [STABLE_EXE, f'Network_Toolkit_{current}_Portable.zip', f'Network_Toolkit_{current}_Update.zip',
            *([windows_setup_name(current)] if has_windows_setup(current) else []),
            *[linux_archive_name(current, arch) for arch, _ in LINUX_ARCHES], 'SHA256SUMS']


def require(condition, message):
    if not condition:
        raise ValueError(message)


def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True).strip()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def release_channel():
    """Choose publication metadata from reviewed source, never from tag text."""
    path = ROOT / 'docs/RELEASE_CHANNEL.txt'
    channel = path.read_text(encoding='utf-8').strip() if path.is_file() else 'stable'
    require(channel in ('stable', 'beta'), 'Invalid release channel marker')
    return channel


def version():
    match = re.search(r'^const Version = "(' + VERSION_PATTERN + r')"$',
                      (ROOT / 'internal/model/model.go').read_text(), re.M)
    require(match is not None, 'Cannot read the application version')
    return match.group(1)


def validate(tag=None):
    current = version()
    channel = release_channel()
    updater = (ROOT / 'cmd/network-toolkit/updater.go').read_text()
    require('const updateBuildChannel = "' + channel + '"' in updater,
            'The application release channel and publication marker differ')
    require(all(int(part) < 65536 for part in current.split('.')), 'Version exceeds Windows resource limits')
    if tag:
        require(tag == 'v' + current, f'Tag {tag!r} must match application version v{current}')
    html = (ROOT / 'web/index.html').read_text()
    require(re.search(r"const VERSION=['\"]" + re.escape(current) + r"['\"];", html), 'UI version differs from Go version')
    executable = f'Network_Toolkit_{current}_Windows_x64.exe'
    for name in ['build.sh', 'BUILD_WINDOWS.cmd', 'Open_Terminal.cmd']:
        require('Network_Toolkit.exe' in (ROOT / name).read_text(), f'Stable executable name missing in {name}')
    resources = runpy.run_path(str(ROOT / 'scripts/build_windows_resources.py'))['version_info']()
    version_words = windows_version_words(current)
    fixed = struct.pack('<6I', 0xFEEF04BD, 0x00010000, *version_words, *version_words)
    require(fixed in resources, 'Numeric Windows resource version differs from Go version')
    require((current + '.0').encode('utf-16le') in resources, 'Windows resource text differs from Go version')
    require('Network_Toolkit.exe'.encode('utf-16le') in resources, 'Windows resource executable name differs')
    require(resources in (ROOT / 'cmd/network-toolkit/resource_windows_amd64.syso').read_bytes(),
            'Regenerate resource_windows_amd64.syso with scripts/build_windows_resources.py before pushing this version')
    generator = runpy.run_path(str(ROOT / 'scripts/build_windows_resources.py'))
    for output, description, original, _ in generator['PROGRAMS'][1:]:
        program = generator['version_info'](description, original)
        require(fixed in program and (current + '.0').encode('utf-16le') in program,
                f'Windows resource version of {original} differs from Go version')
        require((original + '\0').encode('utf-16le') in program, f'Windows resource name of {original} differs')
        require(program in (ROOT / output).read_bytes(),
                f'Regenerate {output} with scripts/build_windows_resources.py before pushing this version')
    require((ROOT / f'RELEASE_NOTES_{current}.txt').is_file(), 'Current release notes are missing')
    readme = (ROOT / 'docs/README.txt').read_text()
    require(readme.startswith('NETWORK TOOLKIT ' + current + '\n'), 'README current version differs')
    require('CURRENT RELEASE - ' + current in readme, 'README current release differs')
    return current


def gh_json(path, missing=False):
    result = subprocess.run(['gh', 'api', path], text=True, capture_output=True)
    if result.returncode:
        if missing and 'HTTP 404' in result.stderr:
            return None
        raise ValueError('GitHub request failed: ' + result.stderr.strip())
    return json.loads(result.stdout)


def remote_tag(repo, tag):
    item = gh_json(f'repos/{repo}/git/ref/tags/{tag}', missing=True)
    if item is None:
        return None
    ref = item['object']
    for _ in range(10):
        if ref['type'] != 'tag':
            require(ref['type'] == 'commit', 'Release tag does not resolve to a commit')
            return ref['sha']
        ref = gh_json(f'repos/{repo}/git/tags/{ref["sha"]}')['object']
    raise ValueError('Release tag nesting is too deep')


def prepare(tag, default_branch, revision=False, expected_old_commit="", expected_old_manifest="", expected_new_commit="", request_branch=False):
    event = os.environ.get('GITHUB_EVENT_NAME', '')
    ref = os.environ.get('GITHUB_REF', '')
    main_push = event == 'push' and ref == 'refs/heads/' + default_branch
    require(not git('status', '--porcelain'), 'Release checkout has uncommitted changes')
    if revision:
        require((event == 'workflow_dispatch' and ref == 'refs/heads/' + default_branch) or
                (request_branch and event == 'push' and ref == 'refs/heads/release-revision-v1.17.63'),
                'Revision requires explicit manual dispatch or the bounded revision request')
        current = validate(tag)
        import revise_release
        revise_release.snapshot(os.environ['GITHUB_REPOSITORY'], tag, expected_old_commit, expected_old_manifest)
        commit = git('rev-parse', 'HEAD')
        require(re.fullmatch(r'[0-9a-f]{40}', expected_new_commit) and commit == expected_new_commit and commit != expected_old_commit, 'Revision requires the explicitly selected new source commit')
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
            output.write(f'publish=true\ntag={tag}\nversion={current}\ncommit={commit}\nrevision=true\nold_commit={expected_old_commit}\nold_manifest={expected_old_manifest}\nnew_commit={expected_new_commit}\n')
        return
    if main_push:
        current = validate()
        tag = 'v' + current
        commit = git('rev-parse', 'HEAD')
        existing = gh_json(f'repos/{os.environ["GITHUB_REPOSITORY"]}/releases/tags/{tag}', missing=True)
        if existing is not None:
            with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
                output.write(f'publish=false\ntag={tag}\nversion={current}\ncommit={commit}\n')
            print(f'{tag} is already published; source checks still run in the Checks workflow')
            return
        existing_tag = remote_tag(os.environ['GITHUB_REPOSITORY'], tag)
        require(existing_tag is None or existing_tag == commit, 'Existing release tag points to another source commit')
    elif event == 'push':
        require(re.fullmatch('v' + VERSION_PATTERN, tag), 'Release tag must have the form v1.17.57')
        require(os.environ.get('GITHUB_REF') == 'refs/tags/' + tag, 'Push ref differs from release tag')
    elif event == 'workflow_dispatch':
        require(re.fullmatch('v' + VERSION_PATTERN, tag), 'Manual publishing requires an existing tag such as v1.17.57')
        require(os.environ.get('GITHUB_REF') == 'refs/heads/' + default_branch,
                'Run manual publishing from the default branch and select an existing release tag')
    else:
        raise ValueError('Prepare requires a default-branch push, version-tag push or manual workflow')
    if not main_push:
        commit = git('rev-parse', '--verify', 'refs/tags/' + tag + '^{commit}')
        subprocess.run(['git', 'merge-base', '--is-ancestor', commit, 'origin/' + default_branch], cwd=ROOT, check=True)
        subprocess.run(['git', 'checkout', '--detach', 'refs/tags/' + tag], cwd=ROOT, check=True)
        current = validate(tag)
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
        output.write(f'publish=true\ntag={tag}\nversion={current}\ncommit={commit}\n')
    print(f'Validated {tag} at {commit}')


def prepare_request(path, default_branch):
    require(os.environ.get('GITHUB_EVENT_NAME') == 'push' and
            os.environ.get('GITHUB_REF') == 'refs/heads/release-revision-v1.17.63',
            'One-time revision request is restricted to its exact branch')
    require(os.environ.get('GITHUB_REPOSITORY') == 'TechSavage1989/network-toolkit',
            'One-time revision is restricted to the main repository')
    request_commit = git('rev-parse', 'HEAD')
    require(request_commit == os.environ.get('GITHUB_SHA'), 'Request checkout differs from triggering commit')
    request = json.loads(Path(path).read_text())
    require(set(request) == {'tag', 'old_commit', 'old_manifest', 'new_commit'}, 'Unexpected revision request fields')
    require(request['tag'] == 'v1.17.63' and
            request['old_commit'] == '8df9b401111e09767591491bc973b4160a17a9bc' and
            request['old_manifest'] == '513dbd928837fe23785cf89a0bcbdb3d94613d7b759efcd0884459fc4116225f',
            'One-time revision differs from the authorized original release')
    require(re.fullmatch(r'[0-9a-f]{40}', request['new_commit']), 'Invalid new source commit')
    head = gh_json('repos/TechSavage1989/network-toolkit/git/ref/heads/main')['object']['sha']
    require(head == request['new_commit'], 'Revision source is no longer main')
    subprocess.run(['git', 'merge-base', '--is-ancestor', head, 'HEAD'], cwd=ROOT, check=True)
    require(git('diff', '--name-only', head, 'HEAD') == '.github/revision-request.json',
            'Revision request branch must change only its request file')
    subprocess.run(['git', 'checkout', '--detach', head], cwd=ROOT, check=True)
    prepare(request['tag'], default_branch, True, request['old_commit'],
            request['old_manifest'], request['new_commit'], request_branch=True)
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
        output.write(f'request_commit={request_commit}\n')


def check_exe(data, current):
    require(len(data) >= 128 and data[:2] == b'MZ', 'Release executable is not a Windows PE file')
    at = struct.unpack_from('<I', data, 0x3c)[0]
    require(at + 94 <= len(data) and data[at:at+4] == b'PE\0\0', 'Invalid PE header')
    require(struct.unpack_from('<H', data, at+4)[0] == 0x8664, 'Executable is not Windows x64')
    require(struct.unpack_from('<H', data, at+24)[0] == 0x20b, 'Executable is not PE32+')
    require(struct.unpack_from('<H', data, at+24+68)[0] == 2, 'Executable is not the Windows GUI build')
    require((current + '.0').encode('utf-16le') in data, 'Executable version resource does not match the release')


def check_program(data, current, name):
    """A separate program: the GUI build of this version with its own original file name."""
    check_exe(data, current)
    require((name + '\0').encode('utf-16le') in data, f'{name} does not carry its own version resource')
    others = {STABLE_EXE, *SEPARATE_PROGRAMS} - {name}
    require(not any((other + '\0').encode('utf-16le') in data for other in others), f'{name} carries another program\'s version resource')


def check_windows_setup(data, current):
    require(len(data) >= 128 and data[:2] == b'MZ', 'Windows setup is not a PE file')
    at = struct.unpack_from('<I', data, 0x3c)[0]
    require(at + 94 <= len(data) and data[at:at+4] == b'PE\0\0', 'Invalid Windows setup PE header')
    require(struct.unpack_from('<H', data, at+24+68)[0] == 2, 'Windows setup is not a GUI executable')
    version_words = windows_version_words(current)
    # NSIS 3.09 writes zero in the structure-version field. Check both the
    # numeric file version and product version independently of that field.
    require(any(struct.pack('<6I', 0xFEEF04BD, structure, *version_words, *version_words) in data
                for structure in (0, 0x00010000)), 'Windows setup version resource does not match the release')


def windows_version_words(current):
    major, minor, patch = map(int, current.split('.'))
    return major << 16 | minor, patch << 16


def package_windows_setup(exe, current, output, programs=None):
    setup = output / windows_setup_name(current)
    temporary = setup.with_name(setup.name + '.building')
    option = '/' if os.name == 'nt' else '-'
    programs = programs or {}
    require(set(programs) == set(SEPARATE_PROGRAMS), 'Windows setup needs the separate Text Editor and Terminal programs')
    try:
        subprocess.run(['makensis', option + 'WX', f'{option}DAPP_VERSION={current}', f'{option}DAPP_EXE={exe.resolve()}',
                        f'{option}DEDITOR_EXE={programs[EDITOR_EXE].resolve()}', f'{option}DTERMINAL_EXE={programs[TERMINAL_EXE].resolve()}',
                        f'{option}DSETUP_EXE={temporary.resolve()}', f'{option}DAPP_ICON={ROOT / "assets/Network_Toolkit_Icon.ico"}',
                        str(ROOT / 'scripts/installer/windows.nsi')], check=True, cwd=ROOT)
        check_windows_setup(temporary.read_bytes(), current)
        os.replace(temporary, setup)
    finally:
        temporary.unlink(missing_ok=True)
    return setup


def source_files(allow_untracked):
    try:
        paths = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT, stderr=subprocess.DEVNULL).decode().split('\0')
        paths = [ROOT / name for name in paths if name]
    except (subprocess.CalledProcessError, FileNotFoundError):
        paths = []
    if not paths:
        require(allow_untracked, 'Commit the source first, or explicitly allow a local untracked-source package')
        paths = list(ROOT.rglob('*'))
    result = []
    for path in sorted(paths):
        relative = path.relative_to(ROOT)
        if {'.git', 'build', 'dist', 'public-dist', 'validation', 'previous-release', '__pycache__'}.intersection(relative.parts):
            continue
        if path.name in {'SHA256SUMS', 'SHA256SUMS.txt'} or path.name.startswith('.env'):
            continue
        require(not path.is_symlink(), f'Source symlink is not supported: {relative}')
        if path.is_file():
            result.append(path)
    require(result, 'Source package would be empty')
    return result


def atomic_write(path, data):
    temporary = path.with_name(path.name + '.building')
    temporary.write_bytes(data)
    os.replace(temporary, path)


def sums(items):
    return ''.join(f'{sha(data)}  {name}\n' for name, data in sorted(items.items())).encode()


def package(exe, output, tag, allow_untracked, linux=None, programs=None):
    current = validate(tag)
    linux = linux or {}
    require(set(linux) == {arch for arch, _ in LINUX_ARCHES}, 'Provide the Linux x86_64 and aarch64 executables')
    separate = has_separate_programs(current)
    programs = programs or {}
    require(set(programs) == (set(SEPARATE_PROGRAMS) if separate else set()),
            'Provide the separate Text Editor and Terminal programs' if separate else 'This version has no separate programs')
    output.mkdir(parents=True, exist_ok=True)
    binary = exe.read_bytes()
    check_exe(binary, current)
    program_bytes = {name: path.read_bytes() for name, path in programs.items()}
    for name, data in program_bytes.items():
        check_program(data, current, name)
    prefix = f'Network_Toolkit_{current}'
    versioned_exe = STABLE_EXE
    terminal_command = f'start "" "%~dp0{TERMINAL_EXE}"' if separate else f'start "" "%~dp0{versioned_exe}" --terminal'
    entries = {versioned_exe: binary, **program_bytes,
               'Open_Terminal.cmd': ('@echo off\r\nsetlocal DisableDelayedExpansion\r\ncd /d "%~dp0"\r\n'
                                     f'{terminal_command}\r\n').encode()}
    for name in ['README.txt', f'RELEASE_NOTES_{current}.txt', 'VALIDATION.txt']:
        entries[name] = (ROOT / name if name.startswith('RELEASE_NOTES_') else ROOT / 'docs' / name).read_bytes()
    entries['SHA256SUMS'] = sums(entries)
    sources = {path.relative_to(ROOT).as_posix(): path.read_bytes() for path in source_files(allow_untracked)}
    sources['SHA256SUMS'] = sums(sources)
    entries.update({'source/' + name: data for name, data in sources.items()})
    archive = output / f'{prefix}_Portable.zip'
    temporary = archive.with_name(archive.name + '.building')
    with zipfile.ZipFile(temporary, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=6, allowZip64=False) as bundle:
        for name, data in sorted(entries.items()):
            info = zipfile.ZipInfo(prefix + '/' + name, (2026, 1, 1, 0, 0, 0))
            info.create_system = 0
            info.external_attr = 0x20
            bundle.writestr(info, data, compress_type=zipfile.ZIP_DEFLATED, compresslevel=6)
    with zipfile.ZipFile(temporary) as bundle:
        require(bundle.testzip() is None, 'Portable ZIP CRC verification failed')
        require(len(bundle.namelist()) == len(entries), 'Portable ZIP has missing or duplicate entries')
        for name, data in entries.items():
            require(bundle.read(prefix + '/' + name) == data, f'Portable ZIP differs from source: {name}')
    os.replace(temporary, archive)
    atomic_write(output / STABLE_EXE, binary)
    import package_public_update
    update_archive = package_public_update.package(exe, output, write_manifest=False, programs=programs or None)
    packages = [output / STABLE_EXE, archive, update_archive]
    if has_windows_setup(current):
        packages.append(package_windows_setup(exe, current, output, programs) if separate else package_windows_setup_legacy(exe, current, output))
    packages.extend(package_linux(linux[arch].read_bytes(), current, arch, machine, output) for arch, machine in LINUX_ARCHES)
    atomic_write(output / 'SHA256SUMS', sums({path.name: path.read_bytes() for path in packages}))
    verify(output, tag)
    print(json.dumps({'version': current, 'archive': str(archive), 'bytes': archive.stat().st_size,
                      'sha256': sha(archive.read_bytes()), 'members': len(entries)}))


def package_windows_setup_legacy(exe, current, output):
    """Setup of a version before the separate programs: never built by this checkout's script."""
    raise ValueError(f'Windows setup of {current} predates the separate programs; build it from its own release source')


def check_linux_exe(data, current, machine):
    require(data[:4] == b'\x7fELF' and data[4] == 2 and data[5] == 1, 'Linux executable is not a 64-bit little-endian ELF file')
    require(struct.unpack_from('<H', data, 18)[0] == machine, 'Linux executable architecture differs from its archive name')
    require(struct.unpack_from('<H', data, 16)[0] in (2, 3), 'Linux file is not an executable')
    require(current.encode() in data, 'Linux executable version differs from the release version')


def package_linux(binary, current, arch, machine, output):
    check_linux_exe(binary, current, machine)
    prefix = f'Network_Toolkit_{current}_Linux_{arch}'
    entries = {LINUX_EXE: binary,
               'README.txt': (ROOT / 'docs' / 'LINUX_README.txt').read_bytes(),
               f'RELEASE_NOTES_{current}.txt': (ROOT / f'RELEASE_NOTES_{current}.txt').read_bytes(),
               'VALIDATION.txt': (ROOT / 'docs' / 'VALIDATION.txt').read_bytes()}
    entries['SHA256SUMS'] = sums(entries)
    archive = output / linux_archive_name(current, arch)
    raw = io.BytesIO()
    with gzip.GzipFile(fileobj=raw, mode='wb', compresslevel=9, mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode='w', format=tarfile.USTAR_FORMAT) as bundle:
            folder = tarfile.TarInfo(prefix)
            folder.type, folder.mode, folder.mtime = tarfile.DIRTYPE, 0o755, LINUX_MTIME
            bundle.addfile(folder)
            for name, data in sorted(entries.items()):
                info = tarfile.TarInfo(prefix + '/' + name)
                info.size, info.mtime = len(data), LINUX_MTIME
                info.mode = 0o755 if name == LINUX_EXE else 0o644
                info.uid = info.gid = 0
                info.uname = info.gname = ''
                bundle.addfile(info, io.BytesIO(data))
    atomic_write(archive, raw.getvalue())
    verify_linux(archive, current, arch, machine)
    return archive


def verify_linux(archive, current, arch, machine):
    prefix = f'Network_Toolkit_{current}_Linux_{arch}'
    with tarfile.open(archive, mode='r:gz') as bundle:
        members = bundle.getmembers()
        names = [m.name for m in members]
        require(len(names) == len(set(names)), 'Linux archive has duplicate entries')
        require(members and members[0].name == prefix and members[0].isdir(), 'Linux archive must start with its folder')
        files = {}
        for member in members[1:]:
            require(member.name.startswith(prefix + '/') and '..' not in Path(member.name).parts and '/' not in member.name[len(prefix) + 1:],
                    f'Unsafe Linux archive member: {member.name}')
            require(member.isfile(), f'Linux archive may contain only regular files: {member.name}')
            name = member.name[len(prefix) + 1:]
            require((member.mode & 0o777) == (0o755 if name == LINUX_EXE else 0o644), f'Unexpected permissions: {name}')
            files[name] = bundle.extractfile(member).read()
        require(LINUX_EXE in files and 'SHA256SUMS' in files, 'Linux archive is missing its program or checksums')
        listed = check_sums(files['SHA256SUMS'], lambda name: files[name])
        require(listed | {'SHA256SUMS'} == set(files), 'Unverified entries found in Linux archive')
        check_linux_exe(files[LINUX_EXE], current, machine)
    return files[LINUX_EXE]


def check_sums(manifest, load):
    names = set()
    for line in manifest.decode().splitlines():
        match = re.fullmatch(r'([0-9a-f]{64})  (.+)', line)
        require(match is not None, 'Invalid checksum manifest')
        digest, name = match.groups()
        require(name not in names and not name.startswith('/') and '..' not in Path(name).parts,
                'Unsafe or duplicate checksum name')
        names.add(name)
        require(sha(load(name)) == digest, f'Checksum failed: {name}')
    require(names, 'Empty checksum manifest')
    return names


def verify(output, tag=None):
    current = validate(tag)
    archive = output / f'Network_Toolkit_{current}_Portable.zip'
    release_names = check_sums((output / 'SHA256SUMS').read_bytes(), lambda name: (output / name).read_bytes())
    update_archive = output / f'Network_Toolkit_{current}_Update.zip'
    require(release_names == set(release_assets(current)) - {'SHA256SUMS'},
            'Unexpected release assets in checksum manifest')
    if has_windows_setup(current):
        check_windows_setup((output / windows_setup_name(current)).read_bytes(), current)
    for arch, machine in LINUX_ARCHES:
        verify_linux(output / linux_archive_name(current, arch), current, arch, machine)
    import package_public_update
    check_exe((output / STABLE_EXE).read_bytes(), current)
    prefix = f'Network_Toolkit_{current}/'
    separate = has_separate_programs(current)
    with zipfile.ZipFile(archive) as bundle:
        programs = {name: bundle.read(prefix + name) for name in SEPARATE_PROGRAMS} if separate else None
    for name, data in (programs or {}).items():
        check_program(data, current, name)
    package_public_update.verify_archive(update_archive, (output / STABLE_EXE).read_bytes(), current, programs)
    with zipfile.ZipFile(archive) as bundle:
        require(bundle.testzip() is None, 'Portable ZIP CRC verification failed')
        members = bundle.namelist()
        require(len(members) == len(set(members)), 'Portable ZIP has duplicate entries')
        require(all(name.startswith(prefix) and '..' not in Path(name).parts for name in members), 'Unsafe ZIP member')
        root_names = check_sums(bundle.read(prefix + 'SHA256SUMS'), lambda name: bundle.read(prefix + name))
        source_names = check_sums(bundle.read(prefix + 'source/SHA256SUMS'), lambda name: bundle.read(prefix + 'source/' + name))
        expected = {prefix + name for name in root_names | {'SHA256SUMS'}}
        expected.update(prefix + 'source/' + name for name in source_names | {'SHA256SUMS'})
        require(set(members) == expected, 'Unverified entries found in portable ZIP')
        executables = {name[len(prefix):] for name in members if name.lower().endswith('.exe')}
        if separate:
            require(executables == {STABLE_EXE, *SEPARATE_PROGRAMS},
                    'Portable ZIP must contain exactly Network_Toolkit.exe and the separate Text Editor and Terminal programs')
        else:
            require(sum(name.lower().endswith('.exe') for name in members) == 1, 'Portable ZIP must contain exactly one executable')
        require(bundle.read(prefix + STABLE_EXE) == (output / STABLE_EXE).read_bytes(),
                'Standalone and portable executables differ')
    return current


def publish(output, repo, tag):
    require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo), 'Repository must be OWNER/NAME')
    current = verify(output, tag)
    local_commit = git('rev-parse', 'HEAD')
    tag_commit = remote_tag(repo, tag)
    require(tag_commit is None or tag_commit == local_commit, 'Published tag differs from the validated source checkout')
    require(gh_json(f'repos/{repo}/releases/tags/{tag}', missing=True) is None,
            'This release already exists; refusing to replace it')
    command = ['gh', 'release', 'create', tag, '--repo', repo, '--target', local_commit,
               '--title', 'Network Toolkit ' + current + (' Beta' if release_channel() == 'beta' else ''),
               '--notes-file', str(ROOT / f'RELEASE_NOTES_{current}.txt')]
    if release_channel() == 'beta':
        command.append('--prerelease')
    command.extend(str(output / name) for name in release_assets(current))
    subprocess.run(command, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    for name in ['validate', 'prepare', 'package', 'verify', 'publish']:
        command = sub.add_parser(name)
        command.add_argument('--tag', required=name == 'publish', default='')
        if name in {'package', 'verify', 'publish'}:
            command.add_argument('--output-dir', type=Path, default=ROOT / 'dist')
        if name == 'prepare':
            command.add_argument('--default-branch', required=True)
            command.add_argument('--revision', action='store_true')
            command.add_argument('--revision-request', default='')
            command.add_argument('--expected-old-commit', default='')
            command.add_argument('--expected-old-manifest', default='')
            command.add_argument('--expected-new-commit', default='')
        if name == 'package':
            command.add_argument('--exe', type=Path, required=True)
            command.add_argument('--editor-exe', type=Path, help='Network_Toolkit_TextEditor.exe (versions with separate programs)')
            command.add_argument('--terminal-exe', type=Path, help='Network_Toolkit_Terminal.exe (versions with separate programs)')
            for arch, _ in LINUX_ARCHES:
                command.add_argument('--linux-' + arch.replace('_', '-'), type=Path, required=True, dest='linux_' + arch,
                                     help=f'Linux {arch} executable')
            command.add_argument('--allow-untracked-source', action='store_true', help='Explicit local setup/testing only')
        if name == 'publish':
            command.add_argument('--repo', required=True)
    args = parser.parse_args()
    if args.command == 'validate':
        print(validate(args.tag))
    elif args.command == 'prepare':
        if args.revision_request:
            prepare_request(args.revision_request, args.default_branch)
        else:
            prepare(args.tag, args.default_branch, args.revision, args.expected_old_commit, args.expected_old_manifest, args.expected_new_commit)
    elif args.command == 'package':
        programs = {name: path for name, path in ((EDITOR_EXE, args.editor_exe), (TERMINAL_EXE, args.terminal_exe)) if path}
        package(args.exe, args.output_dir.resolve(), args.tag, args.allow_untracked_source,
                {arch: getattr(args, 'linux_' + arch) for arch, _ in LINUX_ARCHES}, programs)
    elif args.command == 'verify':
        print('Verified ' + verify(args.output_dir.resolve(), args.tag))
    else:
        publish(args.output_dir.resolve(), args.repo, args.tag)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, zipfile.BadZipFile) as error:
        print('Release stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
