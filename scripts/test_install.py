#!/usr/bin/env python3
"""Offline installer checks; no network, account, or installed CLI is touched."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class InstallTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.bin = self.work / "bin"
        self.bin.mkdir()
        self.home = self.work / "home"
        self.home.mkdir()
        self.target = self.home / ".local/bin/maolv"
        self.asset = self.work / "asset"
        self.asset.write_text('#!/bin/sh\nprintf \'{"data":{"version":"v0.1.0"}}\\n\'\n')
        self.env = dict(os.environ, HOME=str(self.home), PATH=f"{self.bin}:/usr/bin:/bin",
                        TEST_WORK=str(self.work), MAOLV_VERSION="v0.1.0")
        self.executable("uname", '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; esac\n')
        self.executable("curl", """#!/usr/bin/env python3
import json, os, pathlib, shutil, sys
w = pathlib.Path(os.environ['TEST_WORK'])
with (w / 'requests').open('a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')
url = next(a for a in sys.argv[1:] if a.startswith('https://'))
out = sys.argv[sys.argv.index('-o') + 1]
if os.environ.get('TEST_FAIL_DOWNLOAD') and not url.endswith('/checksums.txt'): sys.exit(22)
shutil.copyfile(w / ('checksums.txt' if url.endswith('/checksums.txt') else 'asset'), out)
""")
        self.checksum()

    def executable(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def checksum(self, checksum=None, duplicate=False):
        checksum = checksum or hashlib.sha256(self.asset.read_bytes()).hexdigest()
        line = f"{checksum}  maolv_v0.1.0_linux_amd64\n"
        (self.work / "checksums.txt").write_text(line * (2 if duplicate else 1))

    def run_installer(self):
        return subprocess.run(["/bin/sh", str(ROOT / "scripts/install.sh")], env=self.env,
                              text=True, capture_output=True)

    def test_install_and_preserve(self):
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.target.read_bytes(), self.asset.read_bytes())
        calls = (self.work / "requests").read_text().splitlines()
        self.assertEqual(len(calls), 2)
        for line in calls:
            args = json.loads(line)
            self.assertEqual(args[args.index("--proto") + 1], "=https")
            self.assertEqual(args[args.index("--proto-redir") + 1], "=https")
            self.assertIn("-fLsS", args)
        self.assertEqual(self.run_installer().returncode, 0)
        self.assertEqual((self.work / "requests").read_text().splitlines(), calls)

    def test_checksum_failures(self):
        for digest, duplicate in [("0" * 64, False), (None, True)]:
            with self.subTest(duplicate=duplicate):
                self.checksum(digest, duplicate)
                self.assertNotEqual(self.run_installer().returncode, 0)
                self.assertFalse(self.target.exists())

    def test_download_failure(self):
        self.env["TEST_FAIL_DOWNLOAD"] = "1"
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse(self.target.exists())

    def test_version_mismatch(self):
        self.asset.write_text('#!/bin/sh\necho \'{"data":{"version":"v9.9.9"}}\'\n')
        self.checksum()
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse(self.target.exists())

    def test_reject_version_injection_before_download(self):
        for value in ["../main", "v01.0.0", "v1.0.0;echo bad", "v1.0.0\n"]:
            with self.subTest(value=value):
                self.env["MAOLV_VERSION"] = value
                self.assertNotEqual(self.run_installer().returncode, 0)
                self.assertFalse((self.work / "requests").exists())

    def test_skill_installers_are_self_contained_copies(self):
        for name in ["install.sh", "install.ps1"]:
            self.assertEqual((ROOT / "scripts" / name).read_bytes(),
                             (ROOT / "skills/maolv/scripts" / name).read_bytes())


if __name__ == "__main__":
    unittest.main()
