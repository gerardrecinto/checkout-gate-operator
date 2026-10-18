#!/usr/bin/env bash
set -euo pipefail
: "${VM_DNS:?Set VM_DNS to a DNS server IP address}"

interface=$(ip -4 route show default | awk '{for (i=1; i<=NF; i++) if ($i == "dev") {print $(i+1); exit}}')
if [[ -z "$interface" ]]; then
  echo "FAIL no default-route interface for VM_DNS" >&2
  exit 1
fi

if [[ -n "${VM_DNS_TRUST_ANCHOR:-}" ]]; then
  if [[ ! -s "$VM_DNS_TRUST_ANCHOR" ]]; then
    echo "FAIL missing DNS trust anchor: $VM_DNS_TRUST_ANCHOR" >&2
    exit 1
  fi
  install -D -m 0644 -- "$VM_DNS_TRUST_ANCHOR" /etc/dnssec-trust-anchors.d/vagrant-root.positive
  systemctl restart systemd-resolved
fi

# A global DNS= drop-in appends to the box's servers and does not override
# per-link DNS routing. resolvectl validates the address before changing it.
resolvectl dns "$interface" "$VM_DNS"
resolvectl domain "$interface" '~.'
resolvectl flush-caches
resolvectl status "$interface"
