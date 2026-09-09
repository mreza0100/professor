#!/usr/bin/env python3
"""Exercise refresh-scope's recursive enumeration failure contract.

The optional positional argument selects the shell implementation.  That lets
reviewers run this same fixture suite against a pre-fix script from git while
the CI default tests the checked-in script.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value), encoding="utf-8")


class RefreshScopeTests(unittest.TestCase):
    script: Path

    def run_scan(
        self,
        project: Path,
        mapping: dict,
        *,
        path_prefix: Path | None = None,
    ) -> subprocess.CompletedProcess[str]:
        map_path = project.parent / "refresh-map.json"
        write_json(map_path, mapping)
        environment = os.environ.copy()
        if path_prefix is not None:
            environment["PATH"] = str(path_prefix) + os.pathsep + environment["PATH"]
        command = ["/bin/bash", str(self.script), "scan", str(project), str(map_path)]
        try:
            return subprocess.run(
                command,
                text=True,
                capture_output=True,
                env=environment,
                check=False,
                timeout=5,
            )
        except subprocess.TimeoutExpired as expired:
            stdout = expired.stdout or b""
            stderr = expired.stderr or b""
            if isinstance(stdout, bytes):
                stdout = stdout.decode(errors="replace")
            if isinstance(stderr, bytes):
                stderr = stderr.decode(errors="replace")
            return subprocess.CompletedProcess(
                command,
                124,
                stdout=stdout,
                stderr=stderr + "refresh-scope fixture timed out\n",
            )

    def make_project(self, base: Path) -> Path:
        project = base / "project"
        (project / ".professor").mkdir(parents=True)
        return project

    def write_manifest(self, project: Path, role: str, value: Path) -> None:
        write_json(
            project / ".professor" / "manifest.json",
            {"interview": {"projects": {role: str(value)}}},
        )

    def test_absolute_recursive_glob_handles_special_paths_and_prunes_hidden(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            project = self.make_project(base)
            backend = base / "backend space & |"
            nested = backend / "nested level"
            nested.mkdir(parents=True)
            mapped = nested / "mapped & |.txt"
            unmapped = nested / "unmapped & |.txt"
            mapped.write_text("mapped\n", encoding="utf-8")
            unmapped.write_text("unmapped\n", encoding="utf-8")
            hidden = backend / ".hidden" / "secret.txt"
            hidden.parent.mkdir()
            hidden.write_text("must stay pruned\n", encoding="utf-8")
            self.write_manifest(project, "backend", backend)
            digest = hashlib.sha256(mapped.read_bytes()).hexdigest()
            result = self.run_scan(
                project,
                {
                    "templates": {
                        "fixture.md": {
                            "sources": {
                                "{project:backend}/nested level/mapped & |.txt": digest
                            }
                        }
                    },
                    "source_globs": ["{project:backend}/**"],
                    "ignore_sources": [],
                },
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(f"UNMAPPED-LIVE {unmapped}", result.stdout)
            self.assertNotIn(str(mapped), result.stdout)
            self.assertNotIn(str(hidden), result.stdout)
            self.assertIn("1 unmapped-live", result.stdout)

    def test_unsupported_recursive_shape_fails_without_clean_summary(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            project = self.make_project(Path(temporary))
            for pattern in ("nested/**/files", "nested/**/files/**"):
                with self.subTest(pattern=pattern):
                    result = self.run_scan(
                        project,
                        {"templates": {}, "source_globs": [pattern], "ignore_sources": []},
                    )
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertIn("UNSUPPORTED-GLOB", result.stderr)
                    self.assertNotIn("refresh-scope: 0 changed", result.stdout)

    def test_failing_find_is_an_enumeration_failure(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            project = self.make_project(base)
            backend = base / "backend"
            backend.mkdir()
            fake_bin = base / "fake-bin"
            fake_bin.mkdir()
            failing_find = fake_bin / "find"
            failing_find.write_text(
                "#!/bin/sh\nprintf '%s\\n' 'find fixture failed' >&2\nexit 42\n",
                encoding="utf-8",
            )
            failing_find.chmod(0o700)
            self.write_manifest(project, "backend", backend)
            result = self.run_scan(
                project,
                {"templates": {}, "source_globs": ["{project:backend}/**"], "ignore_sources": []},
                path_prefix=fake_bin,
            )
            self.assertNotEqual(result.returncode, 0, result.stdout)
            self.assertIn("ENUMERATION-FAILED", result.stderr)
            self.assertNotIn("refresh-scope: 0 changed", result.stdout)

    def test_unresolved_project_role_fails_without_clean_summary(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            project = self.make_project(Path(temporary))
            write_json(project / ".professor" / "manifest.json", {"interview": {"projects": {}}})
            result = self.run_scan(
                project,
                {"templates": {}, "source_globs": ["{project:missing}/**"], "ignore_sources": []},
            )
            self.assertNotEqual(result.returncode, 0, result.stdout)
            self.assertIn("manifest .interview.projects.missing is missing/null", result.stderr)
            self.assertNotIn("refresh-scope: 0 changed", result.stdout)

    def test_empty_recursive_match_is_valid(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            project = self.make_project(base)
            (base / "empty").mkdir()
            result = self.run_scan(
                project,
                {"templates": {}, "source_globs": [str(base / "empty") + "/**"], "ignore_sources": []},
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("0 unmapped-live", result.stdout)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "script",
        nargs="?",
        type=Path,
        default=Path(__file__).with_name("refresh-scope.sh"),
        help="refresh-scope.sh implementation to exercise",
    )
    arguments = parser.parse_args()
    RefreshScopeTests.script = arguments.script.resolve()
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(RefreshScopeTests)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    return 0 if result.wasSuccessful() else 1


if __name__ == "__main__":
    raise SystemExit(main())
