#!/usr/bin/env python3
"""Local verification and bounded maintenance commands; never contacts a VPS."""
import argparse
import http.cookiejar
import signal
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
IMAGE = re.compile(r'ghcr\.io/[a-z0-9][a-z0-9._/-]*:[a-f0-9]{40}$')


def ssh_command(host, user, key_file, known_hosts):
    # A reviewable command plan only: no connection is made by the dry-run.
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9.-]*', host) or not re.fullmatch(r'[a-z_][a-z0-9_-]*', user) or user == 'root':
        raise ValueError('SSH requires an explicit host and dedicated non-root user')
    if not Path(key_file).is_absolute() or not Path(known_hosts).is_absolute():
        raise ValueError('SSH key and known_hosts must use explicit absolute paths')
    return ['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'UserKnownHostsFile=' + known_hosts, '-o', 'IdentitiesOnly=yes', '-o', 'ForwardAgent=no', '-o', 'ConnectTimeout=10', '-i', key_file, user + '@' + host]


def release_plan(api_image, web_image):
    if not IMAGE.fullmatch(api_image) or not IMAGE.fullmatch(web_image):
        raise ValueError('release images must be GHCR references tagged with a full commit SHA')
    if api_image.rsplit(':', 1)[1] != web_image.rsplit(':', 1)[1]:
        raise ValueError('API and web must come from the same commit')
    return ['verify-known-hosts-and-dedicated-ssh-user', 'pull-immutable-images', 'migrate', 'seed', 'start', 'health', 'https-smoke', 'rollback-app-only-on-failed-health-or-smoke']


def simulate_release(action):
    """Exercise release failure boundaries with an injected, non-network executor."""
    for step in ('pull-immutable-images', 'migrate', 'seed'):
        action(step)
    try:
        action('start')
        action('health')
        action('https-smoke')
    except Exception:
        action('rollback-app')
        action('rollback-health')
        raise


class Stack:
    def __init__(self, env_file, project, local=False):
        self.prefix = ['docker', 'compose', '--env-file', str(Path(env_file).absolute()), '-p', project, '-f', str(ROOT / 'deploy/compose.production.yml')]
        if local:
            self.prefix += ['-f', str(ROOT / 'deploy/compose.local.yml')]

    def run(self, *args, **kwargs):
        return subprocess.run(self.prefix + list(args), cwd=ROOT, check=True, **kwargs)


def smoke(base, login, password, snapshot=None, cleanup=False):
    parsed = urllib.parse.urlparse(base)
    if parsed.scheme not in ('http', 'https') or not parsed.hostname:
        raise ValueError('invalid smoke URL')
    cookies = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookies))
    def request(path, data=None, method=None):
        body = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(base.rstrip('/') + '/api/v1' + path, data=body, method=method, headers={'Content-Type': 'application/json', 'Origin': base.rstrip('/')})
        with client.open(req, timeout=10) as response:
            return json.loads(response.read()) if response.status != 204 else None
    request('/health/ready')
    request('/auth/login', {'login': login, 'password': password})
    categories = request('/categories')['data']
    if len(categories) != 13:
        raise ValueError('expected all 13 seed categories')
    if snapshot is not None:
        verify_snapshot(request, snapshot)
        if cleanup:
            request('/notes/' + snapshot['note_id'], method='DELETE')
        print('read-only persistence verification passed')
        return snapshot
    topics = request('/topics?query=goroutines&page_size=100')
    if not topics['data']:
        raise ValueError('goroutines is not searchable')
    matches = [topic for topic in topics['data'] if topic['slug'] == 'goroutines']
    if not matches:
        raise ValueError('expected the stable goroutines topic')
    slug = matches[0]['slug']
    detail = request('/topics/' + slug)['data']
    if len(detail['questions']) < 3 or len(detail['checklist']) < 3:
        raise ValueError('incomplete topic content')
    request('/topics/' + slug + '/progress', {'status': 'learning', 'review_required': True}, 'PUT')
    request('/topics/' + slug + '/bookmark', method='PUT')
    note = request('/topics/' + slug + '/notes', {'body_markdown': 'Smoke: durable local note'})['data']
    request('/topics/' + slug + '/checklist/' + detail['checklist'][0]['id'], {'completed': True}, 'PUT')
    state = request('/topics/' + slug)['data']['user_state']
    if state['status'] != 'learning' or not state['bookmarked'] or not state['review_required'] or not state['checklist'][detail['checklist'][0]['id']]:
        raise ValueError('user mutations did not persist')
    snapshot = {'slug': slug, 'note_id': note['id'], 'note_body': note['body_markdown'], 'checklist_id': detail['checklist'][0]['id'], 'status': 'learning'}
    verify_snapshot(request, snapshot)
    print('API smoke passed: auth, full catalog, goroutines, progress, checklist, bookmark, notes')
    return snapshot


def verify_snapshot(request, snapshot):
    state = request('/topics/' + snapshot['slug'])['data']['user_state']
    if state['status'] != snapshot['status'] or not state['bookmarked'] or not state['review_required'] or not state['checklist'].get(snapshot['checklist_id']):
        raise ValueError('saved progress, checklist or bookmark was lost')
    notes = request('/notes')['data']
    if not any(note['id'] == snapshot['note_id'] and note['body_markdown'] == snapshot['note_body'] for note in notes):
        raise ValueError('saved note was lost')


def interrupted(signum, frame):
    raise KeyboardInterrupt('operation interrupted')


def local_smoke(port, e2e):
    project = 'track-app-smoke-' + secrets.token_hex(6)
    with tempfile.TemporaryDirectory(prefix='track-app-smoke-') as tmp:
        tmp = str(Path(tmp).resolve())
        env_path = Path(tmp) / 'stack.env'
        password = secrets.token_hex(16)
        env_path.write_text('\n'.join(['APP_DOMAIN=localhost', 'API_IMAGE=track-app-api:local', 'WEB_IMAGE=track-app-web:local', 'POSTGRES_PASSWORD=' + password, 'DATABASE_URL=postgres://tracker:' + password + '@postgres:5432/tracker?sslmode=disable', 'BOOTSTRAP_LOGIN=local-smoke', 'BOOTSTRAP_PASSWORD=' + password, 'LOCAL_HTTP_PORT=' + str(port)]) + '\n')
        env_path.chmod(0o600)
        stack = Stack(env_path, project, True)
        try:
            stack.run('up', '-d', '--wait', '--wait-timeout', '120', 'postgres')
            stack.run('run', '--rm', 'migrate')
            stack.run('run', '--rm', 'migrate')
            stack.run('run', '--rm', 'seed')
            stack.run('run', '--rm', 'seed')
            stack.run('run', '--rm', 'bootstrap')
            stack.run('run', '--rm', 'bootstrap')
            stack.run('up', '-d', '--wait', '--wait-timeout', '120', 'api', 'caddy')
            base = 'http://127.0.0.1:' + str(port)
            snapshot = smoke(base, 'local-smoke', password)
            if e2e:
                env = dict(os.environ, E2E_BASE_URL=base, E2E_LOGIN='local-smoke', E2E_PASSWORD=password)
                subprocess.run(['npm', 'run', 'test:e2e'], cwd=ROOT / 'frontend', env=env, check=True)
                # E2E changes this topic's status; verify that exact persisted value later.
                snapshot['status'] = 'mastered'
            stack.run('run', '--rm', 'seed')
            smoke(base, 'local-smoke', password, snapshot)
            stack.run('restart', 'postgres', 'api')
            stack.run('up', '-d', '--wait', '--wait-timeout', '120', 'api', 'caddy')
            smoke(base, 'local-smoke', password, snapshot, cleanup=True)
            print('local production smoke, reseed, restart passed')
        finally:
            # Only this randomly named disposable project is destroyed, never production.
            stack.run('down', '--volumes', '--remove-orphans')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('command', choices=['smoke', 'local-smoke', 'release-dry-run'])
    parser.add_argument('--port', type=int, default=4180)
    parser.add_argument('--e2e', action='store_true')
    args = parser.parse_args()
    if args.command == 'local-smoke':
        local_smoke(args.port, args.e2e)
    elif args.command == 'release-dry-run':
        plan = release_plan(os.environ.get('API_IMAGE', ''), os.environ.get('WEB_IMAGE', ''))
        events = []
        simulate_release(events.append)
        result = {'dry_run': True, 'steps': plan, 'simulated': events, 'ssh_configured': False}
        if os.environ.get('DEPLOY_HOST'):
            result['ssh_command'] = ssh_command(os.environ['DEPLOY_HOST'], os.environ['DEPLOY_USER'], os.environ['DEPLOY_KEY_FILE'], os.environ['DEPLOY_KNOWN_HOSTS'])
            result['ssh_configured'] = True
        print(json.dumps(result, indent=2))
    elif args.command == 'smoke':
        smoke(os.environ.get('SMOKE_URL', 'http://127.0.0.1:4180'), os.environ['SMOKE_LOGIN'], os.environ['SMOKE_PASSWORD'])

if __name__ == '__main__':
    signal.signal(signal.SIGTERM, interrupted)
    try:
        main()
    except (ValueError, KeyError, subprocess.CalledProcessError, urllib.error.URLError, KeyboardInterrupt) as error:
        print('operation failed:', type(error).__name__, file=sys.stderr)
        sys.exit(1)
