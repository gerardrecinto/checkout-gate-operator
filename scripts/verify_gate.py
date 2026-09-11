#!/usr/bin/env python3
"""Smoke-tests a live CheckoutGate against a real cluster.

Applies a CheckoutGate custom resource, polls its .status.verdict until
the controller has reconciled it at least once, and exits non-zero if the
verdict doesn't match what was expected. Meant to run in a CI job or a
release pipeline's post-deploy step, the same "verify the real system
converged to the expected state" pattern a smoke test does after a
production rollout, just against a Kubernetes custom resource instead of
an HTTP endpoint.

Usage:
    python3 scripts/verify_gate.py --gate retail-checkout-gate \
        --namespace default --expect-verdict Pass --timeout 60
"""
from __future__ import annotations

import argparse
import sys
import time
from dataclasses import dataclass

from kubernetes import client, config
from kubernetes.client.exceptions import ApiException

GROUP = "checkout.gerardrecinto.dev"
VERSION = "v1alpha1"
PLURAL = "checkoutgates"


@dataclass
class GateStatus:
    verdict: str
    message: str
    ready_replicas: int


class GateNotReconciledError(Exception):
    """Raised when a gate never picks up a status within the timeout."""


def fetch_status(api: client.CustomObjectsApi, namespace: str, name: str) -> GateStatus | None:
    try:
        obj = api.get_namespaced_custom_object(GROUP, VERSION, namespace, PLURAL, name)
    except ApiException as exc:
        if exc.status == 404:
            return None
        raise

    status = obj.get("status") or {}
    verdict = status.get("verdict")
    if not verdict:
        return None
    return GateStatus(
        verdict=verdict,
        message=status.get("message", ""),
        ready_replicas=status.get("readyReplicas", 0),
    )


def wait_for_verdict(
    api: client.CustomObjectsApi,
    namespace: str,
    name: str,
    timeout_seconds: int,
    poll_interval_seconds: float = 2.0,
) -> GateStatus:
    deadline = time.monotonic() + timeout_seconds
    last_status: GateStatus | None = None

    while time.monotonic() < deadline:
        last_status = fetch_status(api, namespace, name)
        if last_status is not None:
            return last_status
        time.sleep(poll_interval_seconds)

    raise GateNotReconciledError(
        f"CheckoutGate {namespace}/{name} had no .status.verdict after {timeout_seconds}s, "
        f"last observed status: {last_status}"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gate", required=True, help="Name of the CheckoutGate to check")
    parser.add_argument("--namespace", default="default")
    parser.add_argument("--expect-verdict", default=None, help="Fail if the observed verdict doesn't match this")
    parser.add_argument("--timeout", type=int, default=60, help="Seconds to wait for the controller to reconcile")
    args = parser.parse_args()

    try:
        config.load_incluster_config()
    except config.ConfigException:
        config.load_kube_config()

    api = client.CustomObjectsApi()

    try:
        status = wait_for_verdict(api, args.namespace, args.gate, args.timeout)
    except GateNotReconciledError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1

    print(f"{args.namespace}/{args.gate}: verdict={status.verdict} ready_replicas={status.ready_replicas}")
    print(f"  message: {status.message}")

    if args.expect_verdict and status.verdict != args.expect_verdict:
        print(f"FAIL: expected verdict {args.expect_verdict!r}, got {status.verdict!r}", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
