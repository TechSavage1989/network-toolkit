"""Optional local GitHub setup and source publisher. Requires Git and GitHub CLI.

Run only on your own computer after installing these tools. Authentication uses
the GitHub CLI browser flow. This script never asks for a token or stores one.
"""
from pathlib import Path
import argparse
import json
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
OWNER = 'TechSavage1989'
REPO = OWNER + '/network-toolkit'
REMOTE = 'https://github.com/' + REPO + '.git'


def run(*args, capture=False):
    result = subprocess.run(args, cwd=ROOT, text=True, check=True,
                            stdout=subprocess.PIPE if capture else None)
    return result.stdout.strip() if capture else None


def profile():
    for executable, url in [('git', 'https://git-scm.com/downloads'), ('gh', 'https://cli.github.com/')]:
        if not shutil.which(executable):
            raise ValueError(f'Install {executable} first: {url}')
    authenticated = subprocess.run(['gh', 'auth', 'status', '--hostname', 'github.com'], cwd=ROOT,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if authenticated.returncode:
        run('gh', 'auth', 'login', '--hostname', 'github.com', '--git-protocol', 'https', '--web', '--scopes', 'workflow')
    user = json.loads(run('gh', 'api', 'user', capture=True))
    if user['login'].lower() != OWNER.lower():
        raise ValueError(f'Sign in to {OWNER} in GitHub CLI before continuing; current account is {user["login"]}')
    return user


def git(*args, capture=False):
    return run('git', *args, capture=capture)


def configure(user):
    git('config', '--local', 'user.name', user.get('name') or user['login'])
    git('config', '--local', 'user.email', str(user['id']) + '+' + user['login'] + '@users.noreply.github.com')
    git('config', '--local', '--replace-all', 'credential.https://github.com.helper', '')
    git('config', '--local', '--add', 'credential.https://github.com.helper', '!gh auth git-credential')


def check_origin():
    origin = git('remote', 'get-url', 'origin', capture=True)
    allowed = {REMOTE.lower(), REMOTE[:-4].lower(), ('git@github.com:' + REPO + '.git').lower()}
    if origin.lower() not in allowed:
        raise ValueError('Origin does not point to the intended repository: ' + REPO)


def setup():
    user = profile()
    result = subprocess.run(['gh', 'api', 'repos/' + REPO], cwd=ROOT, text=True, capture_output=True)
    if result.returncode:
        if 'HTTP 404' not in result.stderr:
            raise ValueError('Could not check repository: ' + result.stderr.strip())
        run('gh', 'repo', 'create', REPO, '--private', '--description',
            'Portable Network Toolkit application source and release automation')
    else:
        info = json.loads(result.stdout)
        if not info['private']:
            raise ValueError('Setup requires a private repository and will not change an existing repository visibility')
        existing = subprocess.run(['gh', 'api', f'repos/{REPO}/commits?per_page=1'], cwd=ROOT,
                                  text=True, capture_output=True)
        if not existing.returncode:
            raise ValueError('Repository already contains commits. No files were replaced. Use the existing checkout or let ChatGPT upload into it.')
        if 'HTTP 409' not in existing.stderr:
            raise ValueError('Could not verify that the repository is empty: ' + existing.stderr.strip())
    if (ROOT / '.git').exists():
        raise ValueError('This folder already has Git history. Use its existing checkout; setup does not replace history.')
    git('init', '-b', 'main')
    configure(user)
    git('remote', 'add', 'origin', REMOTE)
    git('add', '--all')
    git('commit', '-m', 'Import Network Toolkit source and release automation')
    git('push', '--set-upstream', 'origin', 'main')
    print('Repository created and source pushed: https://github.com/' + REPO)
    print('GitHub Actions will test and publish the current application version.')


def publish():
    user = profile()
    if not (ROOT / '.git').exists():
        raise ValueError('Use the existing repository checkout. This folder has no Git history.')
    check_origin()
    if git('branch', '--show-current', capture=True) != 'main':
        raise ValueError('Switch to the main branch before publishing')
    run(sys.executable, 'scripts/build_windows_resources.py')
    run(sys.executable, 'scripts/github_release.py', 'validate')
    configure(user)
    git('fetch', '--no-tags', 'origin', 'main')
    git('merge-base', '--is-ancestor', 'origin/main', 'HEAD')
    git('add', '--all')
    if subprocess.run(['git', 'diff', '--cached', '--quiet'], cwd=ROOT).returncode:
        from github_release import version
        git('commit', '-m', 'Update Network Toolkit ' + version())
    git('push', 'origin', 'main')
    print('Source pushed. A new application version will be built and published automatically.')
    print('An already published version is checked without replacing its existing release.')
    print('Progress: https://github.com/' + REPO + '/actions')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['setup', 'publish'])
    args = parser.parse_args()
    try:
        setup() if args.command == 'setup' else publish()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print('GitHub setup stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
