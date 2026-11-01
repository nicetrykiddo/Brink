# Installed ZCode interface investigation

The desktop launcher and the bundled CLI are separate entrypoints. On this machine:

```sh
node /usr/lib/zcode/glm/zcode.cjs --help
```

This reports CLI 0.17.0, a headless `--prompt` mode and an `app-server` stdio mode. Reading the bundled implementation establishes the following semantics:

- `session/create` creates a workspace-bound runtime session.
- `session/send` admits a background prompt and returns immediately. Its result is not turn completion.
- `session/stop` aborts the active controller. Its empty response does not wait for background completion.
- Session snapshots/events provide a separate observation path. An adapter must establish that the old turn is no longer active before another send. The runtime itself rejects sends while its active controller exists.
- The wire format resembles JSON-RPC but does not include ACP's required `jsonrpc` field or ACP method semantics. Passing it to the ACP adapter is unsupported.

These are source observations, except for the live capability probe below. No ZCode model prompt, cancellation or desktop intervention has been tested. Merely finding the abort-controller code is insufficient proof of successful provider interruption.

The bundled CLI's default provider-config search does not match its installed packaging layout outside Electron. The probe supplies the installed public catalog and an ephemeral personal-config path explicitly. It masks the real home and temporary directories, mounts the rest of the filesystem read-only, clears inherited credentials, and disables networking. It neither reads the real ZCode SQLite DB nor writes global config.

## Reproduce the capability probe

This development probe requires Python, Bubblewrap and the installed bundled Node runtime; none is a dependency of Brink's Go runtime. It creates no model session and sends no inference request.

```sh
python - <<'PYPROBE'
import json
import os
import queue
import subprocess
import threading
import time
from pathlib import Path

command = [
    'bwrap', '--ro-bind', '/', '/',
    '--tmpfs', str(Path.home()), '--tmpfs', '/tmp',
    '--proc', '/proc', '--dev', '/dev', '--unshare-net',
    '--die-with-parent', '--chdir', '/tmp',
    '--setenv', 'ZCODE_BUILTIN_PROVIDER_CONFIG_FILE',
    '/usr/lib/zcode/config/provider/zcode-builtin.json',
    '--setenv', 'ZCODE_PERSONAL_PROVIDER_CONFIG_FILE',
    '/tmp/provider_config.json',
    '/opt/codex-desktop/resources/node-runtime/bin/node',
    '/usr/lib/zcode/glm/zcode.cjs', 'app-server', '--stdio',
]
process = subprocess.Popen(
    command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL, text=True, env={'PATH': os.environ['PATH']},
)
frames = queue.Queue(maxsize=128)
def read_frames():
    for line in process.stdout:
        frames.put(line)
threading.Thread(target=read_frames, daemon=True).start()
request_id = os.urandom(8).hex()
try:
    process.stdin.write(json.dumps({
        'id': request_id, 'method': 'runtime/capabilities', 'params': {},
    }) + '\n')
    process.stdin.flush()
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        try:
            frame = json.loads(frames.get(timeout=.25))
        except queue.Empty:
            continue
        if frame.get('id') == request_id:
            assert 'result' in frame, 'Capability request failed'
            assert isinstance(frame['result'], dict)
            print('Verified capability response keys:', sorted(frame['result']))
            break
    else:
        raise TimeoutError('No capability response within 10 seconds')
finally:
    process.terminate()
    try:
        process.wait(timeout=2)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
PYPROBE
```

Verified response keys: `historyFirst`, `independentPlanState`, `sessionDirectorySource`. This is evidence of a working isolated app-server connection, not a complete supervision adapter. The next adapter experiment must use an equally isolated local provider and check admission, actual interruption, idle-after-stop and resumed output independently before exposing production control.
