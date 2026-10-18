"""Exercise package failures without changing the host or requiring KVM."""
import os
from pathlib import Path
import subprocess

import pytest


PROVISION = Path(__file__).resolve().parents[1] / "deploy/vagrant/provision.sh"


@pytest.mark.parametrize("failed_phase, expected_status", [("update", 100), ("install", 42)])
def test_package_failure_stops_provisioning(tmp_path, failed_phase, expected_status):
    apt = tmp_path / "apt-get"
    apt.write_text("""#!/usr/bin/env bash
printf '%s\\n' "$*" >> "$APT_CALLS"
if [[ " $* " == *" update "* ]]; then
  if [[ "$FAILED_PHASE" = update ]]; then
    echo "Temporary failure resolving 'mirrors.edge.kernel.org'" >&2
    # apt update normally warns and exits zero unless strict error mode is set.
    [[ " $* " != *" APT::Update::Error-Mode=any "* ]] || exit 100
  fi
elif [[ " $* " == *" install "* ]]; then
  exit 42
fi
""")
    apt.chmod(0o755)
    calls = tmp_path / "apt-calls"
    env = dict(os.environ, PATH=f"{tmp_path}:{os.environ['PATH']}",
               APT_CALLS=str(calls), FAILED_PHASE=failed_phase)
    result = subprocess.run(["bash", str(PROVISION)], env=env,
                            capture_output=True, text=True, timeout=5)
    assert result.returncode == expected_status, result.stderr
    phases = calls.read_text().splitlines()
    if failed_phase == "update":
        assert len(phases) == 1, "Package install must not follow a failed index update"
        assert "Temporary failure resolving" in result.stderr
    else:
        assert len(phases) == 2


@pytest.mark.parametrize("header, expected_status", [
    ("Server: nginx\r\n", 0),
    ("Server: nginx\n", 0),
    ("Server: nginx/1.18.0\r\n", 1),
])
def test_server_header_check_accepts_http_line_endings(tmp_path, header, expected_status):
    curl = tmp_path / "curl"
    curl.write_text('#!/usr/bin/env bash\nprintf "%s" "$HTTP_HEADER"\n')
    curl.chmod(0o755)
    # Exercise the real assertion without installing packages or editing /etc.
    check = next(line for line in PROVISION.read_text().splitlines()
                 if line.startswith("curl -ksI "))
    env = dict(os.environ, PATH=f"{tmp_path}:{os.environ['PATH']}", HTTP_HEADER=header)
    result = subprocess.run(["bash", "-c", check], env=env,
                            capture_output=True, text=True, timeout=5)
    assert result.returncode == expected_status, result.stdout + result.stderr
