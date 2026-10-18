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
`VM_DNS` optionally sets a global systemd-resolved DNS server; it does not
guarantee that guest DNS or outbound NAT connectivity works.

The previous CI run failed to resolve the Ubuntu package mirror inside the
guest. The underlying networking cause remains unverified; these changes
make the error fail promptly and collect evidence for the next KVM run.
macOS cannot run this KVM check. The package failure regression tests run
without a VM:

```bash
pytest -q scripts/test_vagrant_provision.py
```
