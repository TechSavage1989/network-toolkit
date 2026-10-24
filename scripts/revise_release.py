"""Explicit, guarded same-version release repair. Never used by ordinary pushes.

The owner must authorize replacement and tag movement, and a durable copy of
original assets must be retained before dispatch. This tool also takes a local
verified backup; the workflow uploads it before any release mutation.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import github_release as release


def snapshot(repo, tag, expected_commit, expected_manifest):
    release.require(re.fullmatch(r'[0-9a-f]{40}', expected_commit), 'Expected old commit must be a full SHA')
    release.require(re.fullmatch(r'[0-9a-f]{64}', expected_manifest), 'Expected old SHA256SUMS digest is required')
    ref = release.gh_json(f'repos/{repo}/git/ref/tags/{tag}')
    release.require(ref['object']['type'] == 'commit' and ref['object']['sha'] == expected_commit,
                    'Existing lightweight tag changed; refusing revision')
    item = release.gh_json(f'repos/{repo}/releases/tags/{tag}')
    release.require(not item['draft'] and not item['prerelease'], 'Expected a stable published release')
    current = tag.removeprefix('v')
    allowed = {release.STABLE_EXE, f'Network_Toolkit_{current}_Portable.zip',
               f'Network_Toolkit_{current}_Update.zip', 'SHA256SUMS'}
    assets = item['assets']
    names = [a['name'] for a in assets]
    release.require(len(names) == len(set(names)) and set(names) <= allowed and
                    {release.STABLE_EXE, 'SHA256SUMS'} <= set(names), 'Unexpected original asset inventory')
    for asset in assets:
        release.require(asset['state'] == 'uploaded' and asset['size'] > 0 and
                        re.fullmatch(r'sha256:[0-9a-f]{64}', asset.get('digest') or ''),
                        'Original asset has no verified GitHub digest')
    manifest = next(a for a in assets if a['name'] == 'SHA256SUMS')
    release.require(manifest['digest'] == 'sha256:' + expected_manifest, 'Original checksum manifest changed')
    return {'repository': repo, 'tag': tag, 'old_commit': expected_commit,
            'release_id': item['id'], 'body': item.get('body', ''),
            'assets': sorted([{'id': a['id'], 'name': a['name'], 'size': a['size'], 'digest': a['digest']}
                              for a in assets], key=lambda a: a['name'])}


def backup(directory, repo, tag, expected_commit, expected_manifest):
    before = snapshot(repo, tag, expected_commit, expected_manifest)
    release.require(not directory.exists(), 'Backup directory already exists; refusing overwrite')
    directory.mkdir(parents=True)
    subprocess.run(['gh', 'release', 'download', tag, '--repo', repo, '--dir', str(directory)], check=True)
    for asset in before['assets']:
        data = (directory / asset['name']).read_bytes()
        release.require(len(data) == asset['size'] and 'sha256:' + release.sha(data) == asset['digest'],
                        'Backup differs from original release asset')
    release.check_sums((directory / 'SHA256SUMS').read_bytes(), lambda n: (directory / n).read_bytes())
    release.require(snapshot(repo, tag, expected_commit, expected_manifest) == before,
                    'Original release changed while being backed up')
    release.atomic_write(directory / 'release-before.json', (json.dumps(before, indent=2) + '\n').encode())
    return before


def publish(output, directory, repo, tag, expected_commit, expected_manifest, expected_new_commit, expected_request_commit=""):
    release.require(repo == 'TechSavage1989/network-toolkit', 'Revision tool is restricted to the main source repository')
    release.require(re.fullmatch(r'[0-9a-f]{40}', expected_new_commit), 'Expected new commit must be a full SHA')
    current = release.verify(output, tag)
    previous = json.loads((directory / 'release-before.json').read_text())
    release.require(snapshot(repo, tag, expected_commit, expected_manifest) == previous,
                    'Original release changed after backup; refusing replacement')
    for asset in previous['assets']:
        data = (directory / asset['name']).read_bytes()
        release.require(len(data) == asset['size'] and 'sha256:' + release.sha(data) == asset['digest'],
                        'Required original backup is missing or changed')
    commit = release.git('rev-parse', 'HEAD')
    release.require(commit == expected_new_commit and commit != expected_commit, 'Revision source differs from explicitly selected new commit')
    branch = release.gh_json(f'repos/{repo}/git/ref/heads/main')
    release.require(branch['object']['sha'] == commit, 'Main advanced after build; refusing revision')
    if expected_request_commit:
        release.require(re.fullmatch(r'[0-9a-f]{40}', expected_request_commit), 'Invalid request commit')
        request = release.gh_json(f'repos/{repo}/git/ref/heads/release-revision-v1.17.63')
        release.require(request['object']['sha'] == expected_request_commit, 'Revision request advanced after build')
    names = [release.STABLE_EXE, f'Network_Toolkit_{current}_Portable.zip',
             f'Network_Toolkit_{current}_Update.zip', 'SHA256SUMS']
    # SHA256SUMS is last; interrupted uploads cannot pass client checksum checks.
    for name in names:
        subprocess.run(['gh', 'release', 'upload', tag, str(output / name), '--repo', repo, '--clobber'], check=True)
    item = release.gh_json(f'repos/{repo}/releases/tags/{tag}')
    by_name = {a['name']: a for a in item['assets']}
    release.require(item['id'] == previous['release_id'] and set(by_name) == set(names), 'Unexpected revised release inventory')
    for name in names:
        release.require(by_name[name].get('digest') == 'sha256:' + release.sha((output / name).read_bytes()),
                        'Published revision asset failed digest verification')
    # Recheck tag immediately before the explicitly authorized same-version move.
    ref = release.gh_json(f'repos/{repo}/git/ref/tags/{tag}')
    release.require(ref['object']['type'] == 'commit' and ref['object']['sha'] == expected_commit,
                    'Tag changed during replacement; stop and reconcile')
    subprocess.run(['gh', 'api', '--method', 'PATCH', f'repos/{repo}/git/refs/tags/{tag}',
                    '-f', 'sha=' + commit, '-F', 'force=true'], check=True)
    release.require(release.remote_tag(repo, tag) == commit, 'Revised tag verification failed')
    subprocess.run(['gh', 'release', 'edit', tag, '--repo', repo, '--notes-file',
                    str(release.ROOT / f'RELEASE_NOTES_{current}.txt')], check=True)
    print('Verified revised release at ' + commit)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['backup', 'publish'])
    parser.add_argument('--repo', required=True)
    parser.add_argument('--tag', required=True)
    parser.add_argument('--expected-old-commit', required=True)
    parser.add_argument('--expected-old-manifest', required=True)
    parser.add_argument('--expected-new-commit', required=True)
    parser.add_argument('--expected-request-commit', default='')
    parser.add_argument('--backup-dir', type=Path, default=Path('previous-release'))
    parser.add_argument('--output-dir', type=Path, default=Path('dist'))
    args = parser.parse_args()
    release.require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', args.repo), 'Invalid repository')
    release.require(re.fullmatch('v' + release.VERSION_PATTERN, args.tag), 'Invalid tag')
    if args.command == 'backup':
        backup(args.backup_dir, args.repo, args.tag, args.expected_old_commit, args.expected_old_manifest)
    else:
        publish(args.output_dir, args.backup_dir, args.repo, args.tag, args.expected_old_commit, args.expected_old_manifest, args.expected_new_commit, args.expected_request_commit)

if __name__ == '__main__':
    main()
