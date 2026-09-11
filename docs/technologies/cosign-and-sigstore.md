# Cosign and Sigstore: proving an image came from where it says it did

**TL;DR: A vulnerability scan answers "is there a known bad thing inside
this image." Signing answers a completely different question: "did this
exact image actually come from the build I trust, unmodified since."
Cosign, keyless, signs the image using the CI workflow's own identity,
no private key to generate, store, or ever leak.**

## Two different questions, easy to conflate

```text
Trivy scan  ->  "does this image contain a known CVE?"
                 a question about CONTENT

Cosign sign ->  "was this image built and pushed by the workflow
                 I actually trust, and has it been tampered with
                 since?"
                 a question about PROVENANCE
```

A perfectly clean, zero-CVE image can still be the *wrong* image, built
by someone else's pipeline, or pushed by a compromised token, or
swapped out after the real one was pushed. Scanning never catches that
class of problem. Signing does.

## Keyed versus keyless, and why this repo uses keyless

The traditional way to sign anything is a key pair: a private key
that signs, a public key that verifies, and now there's a private key
that exists somewhere, has to be protected, rotated, and never leaked.
Sigstore's keyless flow replaces that with short-lived identity instead
of a long-lived secret:

```text
GitHub Actions workflow run starts
        |
        v
   requests an OIDC token from GitHub, proving
   "I am workflow ci.yml, on commit <sha>, in repo <owner/repo>"
        |
        v
   presents that token to Fulcio (Sigstore's certificate authority)
        |
        v
   Fulcio issues a short-lived signing certificate
   (minutes, not years) bound to that exact workflow identity
        |
        v
   cosign sign uses that ephemeral cert+key to sign the image,
   then the signature (and the fact that it happened) is recorded
   in Rekor, a public, append-only transparency log
        |
        v
   the ephemeral key is discarded, nothing persists to leak later
```

```yaml
# .github/workflows/ci.yml
- name: Sign the image (keyless, tied to this workflow's OIDC identity)
  run: cosign sign --yes "ghcr.io/${{ github.repository }}@${{ steps.push.outputs.digest }}"
```

No `COSIGN_KEY` secret anywhere in this repo's settings, because there
is no long-lived key to store. The identity IS the proof.

## Verifying: checking the claim, not just that a signature exists

Signing without verifying the signature actually gets checked
somewhere is security theater. This repo verifies in two places, on
purpose:

```yaml
# right after signing, in the same CI run
- name: Verify the signature we just wrote
  run: |
    cosign verify \
      --certificate-identity-regexp "^https://github.com/${{ github.repository }}/.github/workflows/ci.yml@.*" \
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
      "ghcr.io/${{ github.repository }}@${{ steps.push.outputs.digest }}"
```

`--certificate-identity-regexp` is the part that actually does the
checking, not "is there a signature" but "is there a signature from
*this exact workflow file, in this exact repo*." A signature from a
different repo's CI, or a different workflow file in the same repo,
correctly fails this check. Sibling repos in this same body of work
(`rollout-sentinel`, `joltrin`) run the identical pattern in their own
release pipelines, same reasoning both places.

## Go deeper

- [docs.sigstore.dev/cosign/signing/overview](https://docs.sigstore.dev/cosign/signing/overview/)
  covers keyed versus keyless in more depth than this doc does.
- [docs.sigstore.dev/logging/overview](https://docs.sigstore.dev/logging/overview/)
  explains Rekor, the transparency log, and why a signature being
  publicly logged is itself a security property, not just an audit
  trail.
