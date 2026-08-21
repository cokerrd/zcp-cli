# zcp v0.0.27 Release Notes

> **Status: scheduled for 31 August 2026. Not yet released.**
> This build is in QA. Test against the items below and report findings before
> the tag is cut. Remove this block as part of tagging, because the release
> workflow publishes this file as the GitHub Release body.

A security update for the Go toolchain and dependencies, VPC support for
instance creation, and clearer output from several commands.

## Security update

The Go toolchain moved from 1.26.5 to 1.26.6 and the dependencies were
refreshed. This clears five vulnerabilities in the Go standard library that were
reachable from the object storage commands, covering `net/url`, `crypto/tls`,
`encoding/xml`, `encoding/asn1` and `net/http`, plus one in `golang.org/x/net`.
A vulnerability scan of the result reports nothing reachable. No command
behaviour changed. Upgrading is the only action needed.

## `instance create` supports VPC and existing networks

`zcp instance create` previously always built a new network, and
`--network-plan` was mandatory. It can now place an instance in a VPC, or attach
networks you already have.

`--network-type` accepts `Isolated` (the default), `L2` and `Vpc`. Anything else
is rejected before the request is sent.

```bash
# Build a new VPC network from a virtual router plan
zcp instance create --name my-vpc-vm --project default-9 --region yul-1 \
  --template ubuntu-2604-lts-1 --plan ca2sl --billing-cycle hourly \
  --network-type Vpc --vr-plan <router-plan> --storage-category pro-nvme

# Attach networks that already exist
zcp instance create --name my-multi-net-vm --project default-9 --region yul-1 \
  --template ubuntu-2604-lts-1 --plan ca2sl --billing-cycle hourly \
  --network-type Vpc --networks net-a,net-b --default-network net-a \
  --storage-category pro-nvme
```

`--default-network` is required once you attach more than one network, and must
name one of the networks in `--networks`. `--network-plan` is now required only
for `Isolated` and `L2` when `--networks` is omitted, and is rejected for `Vpc`.
`--vr-plan` is the reverse: required for `Vpc` unless `--networks` is given, and
rejected for the other two.

## `ip static-nat enable` needs a network

`zcp ip static-nat enable` never worked. The API rejects a static NAT request
that does not name a network, and the command had no way to supply one. It now
takes `--network` alongside `--instance`, and reports the status and message the
API returns.

```bash
zcp ip static-nat enable 1036521143 --instance my-vm --network my-network
```

This changes the flags the command requires. Any script calling it without
`--network` needs updating, though the previous form only ever returned an
error.

## `volume attach` and `volume detach` say what happened

Both commands printed a row of empty fields on success. The API returns only a
status and a message for these operations, with no volume object to tabulate.
They now show that status next to the slugs you passed.

```bash
zcp volume attach bs-001001-0042 --vm my-vm
zcp volume detach bs-001001-0042
```

---

## Installation and upgrade

The install script installs the latest release and upgrades an existing
installation in place.

**Linux / macOS**

```bash
curl -fsSL https://github.com/zsoftly/zcp-cli/releases/latest/download/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://github.com/zsoftly/zcp-cli/releases/latest/download/install.ps1 | iex
```

**Manual download:** grab your platform's binary from the
[Releases](https://github.com/zsoftly/zcp-cli/releases) page, `chmod +x`, and
place it on your `PATH`.

**Verify:**

```bash
zcp version   # zcp version v0.0.27
```

First-time setup after installing:

```bash
zcp profile add default --region yul-1 --project default-9   # prompts for bearer token
zcp auth validate
```
