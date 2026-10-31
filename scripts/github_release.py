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
import zipfile

ROOT = Path(__file__).resolve().parents[1]
VERSION_PATTERN = r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)'
STABLE_EXE = 'Network_Toolkit.exe'


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
    major, minor, patch = map(int, current.split('.'))
    fixed = struct.pack('<6I', 0xFEEF04BD, 0x00010000, major << 16 | minor, patch << 16,
                        major << 16 | minor, patch << 16)
    require(fixed in resources, 'Numeric Windows resource version differs from Go version')
    require((current + '.0').encode('utf-16le') in resources, 'Windows resource text differs from Go version')
    require('Network_Toolkit.exe'.encode('utf-16le') in resources, 'Windows resource executable name differs')
    require(resources in (ROOT / 'cmd/network-toolkit/resource_windows_amd64.syso').read_bytes(),
            'Regenerate resource_windows_amd64.syso with scripts/build_windows_resources.py before pushing this version')
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


def package(exe, output, tag, allow_untracked):
    current = validate(tag)
    output.mkdir(parents=True, exist_ok=True)
    binary = exe.read_bytes()
    check_exe(binary, current)
    prefix = f'Network_Toolkit_{current}'
    versioned_exe = STABLE_EXE
    entries = {versioned_exe: binary,
               'Open_Terminal.cmd': ('@echo off\r\nsetlocal DisableDelayedExpansion\r\ncd /d "%~dp0"\r\n'
                                     f'start "" "%~dp0{versioned_exe}" --terminal\r\n').encode()}
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
    update_archive = package_public_update.package(exe, output, write_manifest=False)
    atomic_write(output / 'SHA256SUMS', sums({STABLE_EXE: binary, archive.name: archive.read_bytes(),
                                           update_archive.name: update_archive.read_bytes()}))
    verify(output, tag)
    print(json.dumps({'version': current, 'archive': str(archive), 'bytes': archive.stat().st_size,
                      'sha256': sha(archive.read_bytes()), 'members': len(entries)}))


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
    require(release_names == {STABLE_EXE, archive.name, update_archive.name},
            'Unexpected release assets in checksum manifest')
    import package_public_update
    package_public_update.verify_archive(update_archive, (output / STABLE_EXE).read_bytes(), current)
    check_exe((output / STABLE_EXE).read_bytes(), current)
    prefix = f'Network_Toolkit_{current}/'
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
    command.extend([str(output / STABLE_EXE), str(output / f'Network_Toolkit_{current}_Portable.zip'),
                    str(output / f'Network_Toolkit_{current}_Update.zip'),
                    str(output / 'SHA256SUMS')])
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
        package(args.exe, args.output_dir.resolve(), args.tag, args.allow_untracked_source)
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
