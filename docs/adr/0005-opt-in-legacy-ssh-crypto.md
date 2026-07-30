# 5. Opt-in legacy SSH crypto, never a default

## Status

Accepted

## Context

Some real switches in the target environment run SSH stacks old enough to
only offer key exchange, cipher, MAC, or host-key algorithms that Go's
`golang.org/x/crypto/ssh` disables by default for good reason (they're
cryptographically weak). The server still needs to reach them.

## Decision

`NetworkDevice.LegacyCrypto` (`legacy_crypto: true` in YAML) is a bool that
is **never enabled by default** and must be opted into per device.
`internal/ssh.applyLegacyCrypto` widens the negotiated algorithm sets using
`golang.org/x/crypto/ssh`'s own `InsecureAlgorithms()`, **additively** —
appended after the modern supported set, never replacing it — so
negotiation still prefers a modern algorithm when the target offers one,
and every other target in the same inventory keeps the secure defaults
untouched. The existence of `InsecureAlgorithms()` on the actual installed
module version was confirmed via `go doc` before relying on it, rather than
assumed from memory.

## Consequences

- A misconfigured or newly-added device is secure by default: reaching a
  legacy device requires a deliberate, visible `legacy_crypto: true` in the
  inventory, not an implicit fallback that could silently downgrade every
  connection.
- The flag is coarse (all-or-nothing legacy algorithm support), not a
  per-algorithm allowlist. This has been sufficient in practice — see the
  worked example of a Cisco 3650 reachable only via
  `diffie-hellman-group14-sha1` key exchange and `ssh-rsa` host keys/pubkey
  signatures, both covered by `InsecureAlgorithms()`'s additions — but a
  device needing an algorithm outside that specific set would need a new,
  more granular option rather than reusing this one.
- `buildAuthMethods` also always registers `ssh.KeyboardInteractive`
  alongside `ssh.Password` whenever a password is configured, since many
  old network-vendor SSH stacks (Cisco IOS included) only offer
  keyboard-interactive auth, not "password" — password-only auth would
  silently fail against them.
