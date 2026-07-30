# 8. SSH public-key auth for routers/switches

## Status

Accepted

## Context

`NetworkDevice`'s original doc comment stated that essentially no
router/switch in the target environment supports public-key auth, so the
type only had a `Password` field — unlike `Server`, which had both `Key`
and `Password` from the start
([ADR 0004](0004-unified-target-abstraction-for-transport.md) unified the
*transport* layer across servers/routers/switches, but this schema gap
remained). That assumption didn't hold once real devices needing key auth
(a switch with an imported authorized key, a MikroTik `/user ssh-keys`
entry) needed to be added to inventory — there was no field to express it.

## Decision

Add `NetworkDevice.Key string` (`key:` in YAML), mirroring `Server.Key`
exactly, and wire it through `networkDeviceTarget` into `Target.Key`. No
change was needed in `internal/ssh`: `buildAuthMethods` already builds
`ssh.PublicKeys` from `Target.Key` when set, and password auth methods when
`Target.Password` is set — both can coexist, and the two were already
independent because that code was written for `Server` first.

## Consequences

- The documented assumption in `NetworkDevice`'s comment ("none support
  public-key auth") is now "most don't, but some do" — a minority case
  handled by the same field servers already use, not a special vendor
  path.
- Confirmed live against three real device classes rather than only
  unit-tested: a Cisco switch reachable over SSH with key auth *and*
  [legacy crypto](0005-opt-in-legacy-ssh-crypto.md) simultaneously, a
  MikroTik router over SSH key auth with modern algorithms, and (for
  contrast) a Telnet-only Cisco router that has no key-auth concept at
  all — same inventory schema, same `run_command`/vendor tool call shape,
  no branching in the tool layer.
- `Key` and `Password` may both be set on one `NetworkDevice` entry
  (`buildAuthMethods` offers both as auth methods); this is intentional —
  a device mid-migration from password to key auth, or where one is a
  fallback, doesn't need two inventory entries.
