"""Create a protected SQLite backup in a fresh temporary home on Windows.

The source and its sidecars must have medium or higher integrity. Check their
labels before use. Windows no-write-up protects them during the SQLite backup.
No original ACL changes, immutable mode, or raw live-file copies are used.
"""
import ctypes as c
from ctypes import wintypes as w
from contextlib import closing
import json
import hashlib
import pathlib
import sqlite3
import subprocess
import sys
import tempfile
import time

source = pathlib.Path(sys.argv[1]).resolve()
root = pathlib.Path(sys.argv[2]).resolve()
if not root.is_relative_to(pathlib.Path(tempfile.gettempdir()).resolve()):
    raise ValueError('Destination must be temporary')
root.mkdir()
home = root / 'home'
home.mkdir()
subprocess.run(['icacls', str(home), '/setintegritylevel', '(OI)(CI)L'], check=True, capture_output=True)
a = c.WinDLL('advapi32', use_last_error=True)
k = c.WinDLL('kernel32', use_last_error=True)
k.GetCurrentProcess.restype = w.HANDLE
for name, args in [
    ('OpenProcessToken', [w.HANDLE, w.DWORD, c.POINTER(w.HANDLE)]),
    ('DuplicateTokenEx', [w.HANDLE, w.DWORD, w.LPVOID, w.DWORD, w.DWORD, c.POINTER(w.HANDLE)]),
    ('ConvertStringSidToSidW', [w.LPCWSTR, c.POINTER(w.LPVOID)]),
    ('SetTokenInformation', [w.HANDLE, w.DWORD, w.LPVOID, w.DWORD]),
    ('SetThreadToken', [c.POINTER(w.HANDLE), w.HANDLE]),
]:
    f = getattr(a, name)
    f.argtypes, f.restype = args, w.BOOL

class Label(c.Structure):
    _fields_ = [('Sid', w.LPVOID), ('Attributes', w.DWORD)]

def check(ok):
    if not ok:
        raise c.WinError(c.get_last_error())

p, token, sid = w.HANDLE(), w.HANDLE(), w.LPVOID()
check(a.OpenProcessToken(k.GetCurrentProcess(), 0x0002 | 0x0008, p))
check(a.DuplicateTokenEx(p, 0xF01FF, None, 2, 2, token))
check(a.ConvertStringSidToSidW('S-1-16-4096', sid))
label = Label(sid, 0x20)
check(a.SetTokenInformation(token, 25, c.byref(label), c.sizeof(label) + 12))
check(a.SetThreadToken(None, token))
try:
    start = time.perf_counter()
    with closing(sqlite3.connect(source.as_uri() + '?mode=ro', uri=True)) as reader, closing(sqlite3.connect(home / 'bus.db')) as writer:
        reader.backup(writer)
    elapsed = (time.perf_counter() - start) * 1000
finally:
    check(a.SetThreadToken(None, None))

db = sqlite3.connect(home / 'bus.db')
summary = {'backupMs': round(elapsed, 3), 'bytes': (home / 'bus.db').stat().st_size}
summary['decisionStartPresent'] = bool(db.execute("SELECT COUNT(*) FROM sqlite_master WHERE name='decision_start'").fetchone()[0])
for table in ['messages', 'sessions', 'deliveries', 'steers', 'steering_submissions', 'person_requests']:
    summary[table] = db.execute('SELECT COUNT(*) FROM ' + table).fetchone()[0]
for table in ['web_codes', 'web_sessions', 'hook_turns', 'decision_previews']:
    db.execute('DELETE FROM ' + table)
db.commit()
digest = hashlib.sha256(repr(db.execute('SELECT * FROM messages ORDER BY id').fetchall()).encode()).hexdigest()
(root / 'message-digest.txt').write_text(digest)
db.close()
(root / 'backup-counts.json').write_text(json.dumps(summary, indent=2))
print(json.dumps(summary))
