# cert-manager and TLS: certificates that renew themselves

**TL;DR: cert-manager is a Kubernetes controller that watches
`Certificate` resources and keeps a Secret full of a valid TLS
key/cert pair, requesting, renewing, and rotating automatically. Point
anything that needs TLS (here, the admission webhook) at that Secret and
never think about expiry again.**

## The problem it solves

Any HTTPS server needs a private key and a certificate. Getting one
manually (`openssl req ...`) is a five-minute task. Getting one that
renews itself before it expires, that gets distributed to every replica
that needs it, that a new component can request without a human running
a script, takes real ongoing attention, exactly the kind of recurring
operational toil a controller is good at absorbing.

```text
Issuer (checkout-gate-selfsigned-issuer)
   "I'm a certificate authority, self-signed, trust me"
        |
        v
Certificate (checkout-gate-webhook-cert)
   "I want a cert for this DNS name, signed by that Issuer,
    store the result in this Secret"
        |
        v
cert-manager's controller notices the Certificate resource,
generates a key pair, requests/self-signs a cert, writes:
   Secret: checkout-gate-webhook-server-cert
     tls.crt, tls.key
        |
        v
   mounted read-only into the manager Pod
        |                                    |
        v                                    v
webhook server reads tls.crt/tls.key    ValidatingWebhookConfiguration's
from the mounted volume, terminates     caBundle is kept in sync via the
TLS with it                             cert-manager.io/inject-ca-from
                                         annotation, so the API server
                                         always trusts whatever cert
                                         is currently valid
```

## Issuer versus ClusterIssuer, and why this repo uses the namespaced one

cert-manager has two resources that can sign certificates: `Issuer`
(namespaced, can only issue certs for Certificates in the same
namespace) and `ClusterIssuer` (cluster-scoped, any namespace can
request from it). This repo uses a namespaced `Issuer`
(`checkout-gate-selfsigned-issuer`, in `config/webhook/certificate.yaml`)
specifically because the webhook cert is the only thing that needs
signing here, scoping the issuer to the same namespace as the thing
using it means there's no cluster-wide trust relationship to reason
about or accidentally reuse for something unrelated later.

## Self-signed here, a real CA in front of a public service

`spec.selfSigned: {}` means this Issuer is its own root of trust, it
signs its own certificates rather than getting them from a public CA
like Let's Encrypt. That's the correct choice for internal
cluster-to-cluster TLS like this (the API server talking to the
webhook), where nothing outside the cluster ever needs to validate the
cert against a public trust store. A public-facing service would swap
this for an `ACMEIssuer` pointed at Let's Encrypt or a corporate CA
instead, same `Certificate` resource shape, different `Issuer` backing
it, the application code touching the resulting Secret doesn't change
at all.

## The annotation that closes the loop

```yaml
metadata:
  annotations:
    cert-manager.io/inject-ca-from: checkout-gate-system/checkout-gate-webhook-cert
```

Without this, the `ValidatingWebhookConfiguration`'s `caBundle` field
would need to be set by hand, and it would go stale the moment
cert-manager rotated the certificate, since the CA bundle the API
server trusts wouldn't match the new cert's signer anymore. This
annotation tells cert-manager's `cainjector` component to keep watching
that Certificate and keep the webhook config's trusted CA bundle
current automatically, the last manual step removed.

## Go deeper

- [cert-manager.io/docs/concepts/certificate](https://cert-manager.io/docs/concepts/certificate/)
  for the Certificate/Issuer resource model in depth.
- [cert-manager.io/docs/concepts/ca-injector](https://cert-manager.io/docs/concepts/ca-injector/)
  for exactly what `cainjector` does with that annotation.
