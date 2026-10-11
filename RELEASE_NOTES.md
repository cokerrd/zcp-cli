# zcp v1.0.1 Release Notes

v1.0.1 is a compatibility patch for the current API routes. It updates the Go
client's instance deletion workflow and hostname-change request.

## Fixed

- `instance.Service.Delete` retains its exported signature and now submits an
  immediate Virtual Machine service-cancellation request. It resolves the VM's
  billing cycle first. The legacy `expunge` argument remains accepted but has
  no effect because cancellation is immediate.
- `instance.Service.ChangeHostname` sends the current instance name and the
  requested hostname to the hostname-change endpoint. Changing the hostname
  restarts the instance.
- `snapshot.Service.List` retrieves every reported page and returns an error
  rather than partial results when a later page fails or pagination metadata is
  inconsistent.
- `zcp volume create` accepts a named `--plan` with a positive `--size` to set
  capacity in GB for the selected storage tier.

## v1.0.0 release material

v1.0.0 requires separate compute and root-storage selections for new instances
and Kubernetes clusters. It adds VM backup schedule commands, restore
preflight checks, lifecycle-state checks for deletion, and complete DNS lists.

Replace every quoted placeholder in the examples with a value from your account.

## Highlights

### Choose compute and root storage separately

New instances use a named compute plan, a named root-storage plan, and an
explicit root-disk capacity. Kubernetes clusters use distinct control-plane,
worker, and root-storage plans. This makes the resources selected for compute
and root storage explicit in the request.

```bash
zcp plan vm --region yul-1
zcp plan storage --region yul-1

zcp instance create \
  --name my-vm \
  --template ubuntu-2604-lts-1 \
  --plan "<compute-plan>" \
  --blockstorage-plan "<root-storage-plan>" \
  --root-disk-size 100 \
  --network-plan "<network-plan>" \
  --storage-category "<storage-category>" \
  --billing-cycle hourly \
  --region yul-1 \
  --project default-9
```

Kubernetes creation now selects each node role and the root volume separately.
The root-storage selection and capacity apply to every cluster node.

```bash
zcp kubernetes create \
  --name my-cluster \
  --version "<kubernetes-version>" \
  --control-plane-plan "<control-plane-plan>" \
  --worker-plan "<worker-plan>" \
  --storage-plan "<root-storage-plan>" \
  --root-disk-size 100 \
  --storage-category "<storage-category>" \
  --workers 3 \
  --billing-cycle hourly \
  --region yul-1 \
  --project default-9 \
  --ssh-key "<ssh-key>"
```

`zcp kubernetes get` now displays reported node resources, root volumes,
network details, and autoscaling status when the API provides them. Use
`--enable-csi` on create to request Cloud Storage Integration for a supported
cluster.

### Manage VM backup schedules

`zcp vm-backup schedule` can create, list, get, update, pause, resume, run-now,
and delete VM backup schedules. It supports hourly, daily, weekly, and monthly
intervals, IANA timezones, retention counts, and an optional immediate backup.

```bash
zcp vm-backup schedule create my-vm \
  --name nightly \
  --interval dailyAt \
  --at 01:00 \
  --timezone America/Toronto \
  --retention 7 \
  --immediate

zcp vm-backup schedule list
zcp vm-backup schedule pause "<schedule-id>"
zcp vm-backup schedule run-now "<schedule-id>"
```

`zcp vm-backup restore <vm-slug> <backup-slug>` overwrites the VM's current
disks. It verifies that the VM is stopped and the backup is ready, belongs to
that VM, and is in the selected region and project before submitting the restore
request. It prompts for confirmation by default; use `--yes` to skip the prompt
when scripting an intentional restore:

```bash
zcp vm-backup restore "<vm-slug>" "<backup-slug>" --yes
```

`zcp vm-backup create` also returns the created schedule slug in table, JSON,
and YAML output.

### Safer lifecycle and DNS behavior

- Instance and Kubernetes deletion commands check the current lifecycle state before
  requesting immediate cancellation. API error envelopes are reported even when
  returned with HTTP 200.
- `zcp dns list` retrieves every reported page and fails rather than returning a
  partial list if a later page fails or its pagination metadata is inconsistent.
- `zcp dns show` uses the matching list entry when the detail response omits a
  domain status. It shows `-` only when the API reports no status.

## Breaking changes and migration

`zcp instance create` no longer accepts `--cpu`, `--memory`, or `--disk` to
select a custom compute plan. Choose a named compute plan with `zcp plan vm`.
Every new instance must also pass both `--blockstorage-plan` and
`--root-disk-size`.

`zcp kubernetes create` now requires `--control-plane-plan`, `--worker-plan`,
`--storage-plan`, and `--root-disk-size`. Replace a former single cluster-plan
selection with plans for both node roles and the shared root-storage tier.

The root-storage plan must be a named available tier. A custom root-storage
plan is not supported by these commands.

## Go library consumers

The Go module adds scheduler-backed VM backup APIs, VM backup restore support,
DNS `FindBySlug`, richer Kubernetes response types, and root-storage fields on
instance and Kubernetes create requests. No exported request fields were
removed. DNS list callers now receive an error rather than a partial result when
pagination fails. Consumers that construct instance or Kubernetes create
requests should populate the separate root-storage fields before moving to this
release.

## Known limitations

- Automatic Kubernetes scale-out is unverified. `zcp kubernetes scale --enable-autoscaling` submits the requested configuration, but the platform may not add a worker. Scaling can also change root-volume subscriptions. Billing correctness after scaling remains unverified.
- The current platform can persist an `everyOtherDay` VM backup policy before
  returning an error. List schedules after an error and remove any unintended
  policy before retrying.
- Kubernetes create/delete and VM backup restore operations run asynchronously.
  A successful command confirms acceptance of the request, not completion of
  the operation. Check the resource's current status and, after a restore, its
  activity logs.

## Installation and upgrade

The install scripts install the latest release and upgrade an existing
installation in place.

**Linux / macOS**

```bash
curl -fsSL https://github.com/zsoftly/zcp-cli/releases/latest/download/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://github.com/zsoftly/zcp-cli/releases/latest/download/install.ps1 | iex
```

**Manual download:** download the binary or archive for your platform from the
[Releases](https://github.com/zsoftly/zcp-cli/releases) page. Extract an archive
first. Rename the platform binary to `zcp` (or `zcp.exe` on Windows). On Linux
or macOS, run `chmod +x zcp`, then place it on your `PATH`.

**Verify:**

```bash
zcp version   # zcp version v1.0.0
```

First-time setup:

```bash
zcp profile add default --region yul-1 --project default-9
zcp auth validate
```
