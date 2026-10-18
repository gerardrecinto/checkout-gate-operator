# Vagrant provisioning check

On a Linux host with KVM, Vagrant, and the `vagrant-libvirt` plugin installed:

```bash
cd deploy/vagrant
vagrant up --provider=libvirt
vagrant destroy -f
```

Provisioning installs NGINX, enables a local TLS health endpoint, and checks
the response, server header, worker user, and TCP syncookies. Package index
errors stop provisioning before package installation; apt network requests
have a 30-second timeout and at most two retries.
The server header check accepts HTTP CRLF line endings while still rejecting
a header that includes the NGINX version.

The GitHub Actions job limits VM startup and provisioning to 15 minutes,
prints guest addresses, routes, resolver configuration, and a repository DNS
lookup on failure, and attempts VM cleanup after success or failure.
`VM_DNS` optionally sets a single DNS server IP address on the guest's
default-route interface and routes all DNS queries through that link. The
resolver rejects invalid addresses, and provisioning stops if the interface
is missing or DNS configuration fails. DNSSEC settings are preserved.

The first diagnostic CI run failed promptly and cleaned up the VM. It showed
that the old global DNS drop-in left inherited global and per-link servers
active, including a different current DNS server from `VM_DNS`. The explicit
per-link override corrects that configuration; successful DNS resolution and
outbound NAT connectivity still need verification in the next Linux run.
macOS cannot run this KVM check. The package failure regression tests run
without a VM:

```bash
pytest -q scripts/test_vagrant_provision.py
```
