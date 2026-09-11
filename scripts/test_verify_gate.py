"""Real pytest coverage for verify_gate.py, mocking only the Kubernetes API
client boundary (kubernetes.client.CustomObjectsApi), not the logic under
test: fetch_status and wait_for_verdict run for real against a fake API
object, same seam-at-the-boundary pattern used throughout this repo's Go
code (MetricsProvider, client.Reader).
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest
from kubernetes.client.exceptions import ApiException

from verify_gate import GateNotReconciledError, fetch_status, wait_for_verdict


def make_api(obj=None, raise_status=None):
    api = MagicMock()
    if raise_status is not None:
        api.get_namespaced_custom_object.side_effect = ApiException(status=raise_status)
    else:
        api.get_namespaced_custom_object.return_value = obj
    return api


def test_fetch_status_returns_none_when_gate_missing():
    api = make_api(raise_status=404)
    assert fetch_status(api, "default", "missing-gate") is None


def test_fetch_status_reraises_non_404_errors():
    api = make_api(raise_status=500)
    with pytest.raises(ApiException):
        fetch_status(api, "default", "some-gate")


def test_fetch_status_returns_none_before_first_reconcile():
    api = make_api(obj={"status": {}})
    assert fetch_status(api, "default", "fresh-gate") is None


def test_fetch_status_parses_a_real_status_payload():
    api = make_api(obj={
        "status": {
            "verdict": "Pass",
            "message": "all thresholds satisfied",
            "readyReplicas": 3,
        }
    })
    status = fetch_status(api, "default", "checkout-gate")
    assert status is not None
    assert status.verdict == "Pass"
    assert status.ready_replicas == 3


def test_wait_for_verdict_returns_as_soon_as_status_appears():
    api = make_api(obj={"status": {"verdict": "Breach", "message": "p99 exceeds max", "readyReplicas": 2}})
    status = wait_for_verdict(api, "default", "checkout-gate", timeout_seconds=5, poll_interval_seconds=0.01)
    assert status.verdict == "Breach"


def test_wait_for_verdict_times_out_if_never_reconciled():
    api = make_api(raise_status=404)
    with pytest.raises(GateNotReconciledError):
        wait_for_verdict(api, "default", "never-created", timeout_seconds=0.1, poll_interval_seconds=0.02)
