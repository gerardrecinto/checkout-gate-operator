# Admission webhooks: the live check a CRD schema can't do

**TL;DR: A CRD's OpenAPI schema can check shape (is this field the right
type, in the right range). It can't check anything that requires
looking at the rest of the cluster. An admission webhook is a real HTTP
server the API server calls, synchronously, on every create or update,
that can say yes, no, or "yes, but here's a warning" based on whatever
live check you write.**

## Where a webhook sits in the request path

```text
kubectl apply -f my-gate.yaml
        |
        v
  API server receives the write
        |
        v
  Schema validation (the CRD's OpenAPI schema, no webhook involved yet)
        |
        v
  Mutating webhooks run (none in this repo, but this is where
  defaulting/normalization would happen if there were any)
        |
        v
  Validating webhooks run  <-- this repo's CheckoutGateValidator lives here
        |
        v
  Object is persisted to etcd, only if every validating webhook said yes
```

The request blocks on this. If the webhook is slow or unreachable, the
`kubectl apply` hangs or fails, which is exactly why
`failurePolicy: Fail` (used in this repo's
[`config/webhook/certificate.yaml`](../../config/webhook/certificate.yaml))
is a real production decision, not a default left alone: it means "if
my webhook can't be reached, reject the write rather than silently
skip validation." The alternative, `Ignore`, is safer for uptime and
weaker for safety. This repo picks `Fail` because the one check this
webhook does (confirming the referenced Deployment situation makes
sense) is worth blocking a bad write over.

## What this repo's webhook actually checks, and why it's not redundant with the CRD schema

[`internal/webhook/checkoutgate_webhook.go`](../../internal/webhook/checkoutgate_webhook.go)'s
`validate` function does one live check the schema fundamentally can't:
it calls `v.Client.Get` to check whether the `CheckoutGate`'s
`targetDeployment` actually exists in the cluster right now. A CRD
schema has no concept of "does another object with this name exist",
it only ever sees the one object being written, in isolation. That's
the entire reason to reach for a webhook instead of adding another
`+kubebuilder:validation` marker: the check needs a live API call, not
just the shape of the object in front of it.

**Mnemonic: LIVE.** Look up the referenced object via the client,
Interpret "not found" as a warning, not a hard rejection (a gate can
legitimately be applied before its target Deployment, common in
GitOps), Verify anything the schema truly cannot express, Error only on
checks that must block the write.

## Real TLS, not a self-issued cert someone forgets to rotate

The API server calls this webhook over HTTPS, and Kubernetes won't
trust just any certificate, it needs to trust the specific CA that
signed the webhook's serving cert. This repo hands that whole lifecycle
to [cert-manager](cert-manager-and-tls.md) instead of managing it by
hand:

```text
cert-manager Issuer + Certificate  -->  Secret (checkout-gate-webhook-server-cert)
                                              |
                          mounted read-only into the manager Pod
                          at /tmp/k8s-webhook-server/serving-certs
                                              |
        cert-manager.io/inject-ca-from annotation on the
        ValidatingWebhookConfiguration automatically keeps the
        API server's trusted CA bundle in sync with whatever
        cert-manager is currently issuing
```

Nobody generates a cert by hand, nobody sets a calendar reminder to
rotate it before it expires, that's the entire point of routing this
through cert-manager instead of a static file.

## Why `ValidateCreate`/`ValidateUpdate`/`ValidateDelete` as three separate methods

`admission.CustomValidator`'s three-method shape exists because the
right check often differs by operation. This repo runs the same
`validate` logic for create and update (a gate's target should exist
and make sense any time its spec changes) but does nothing at all on
delete, there's no meaningful validation to do when something's being
removed. The interface forces a conscious decision about each one instead of
silently applying create-time logic to a delete by accident.

## Go deeper

- [kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
  is the API server's own documentation of the admission chain.
- [book.kubebuilder.io/cronjob-tutorial/webhook-implementation](https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
  walks a validating webhook build in the same shape as this repo's.
