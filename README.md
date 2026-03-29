# hcloud-fs

A FUSE filesystem that mounts your Hetzner Cloud infrastructure as a directory tree.

## Requirements

- macFUSE (macOS): `brew install macfuse`
- FUSE (Linux): `apt install fuse` or equivalent

## Usage

```bash
export HCLOUD_TOKEN=your-token
mkdir -p /tmp/hcloud
./hcloud-fs /tmp/hcloud
```

Unmount with Ctrl-C or `umount /tmp/hcloud`.

## Structure

```
/tmp/hcloud/
  servers/
    <id>/         name*, status*, location, public_ipv4, public_ipv6,
                  server_type, image, labels.json*, metadata.json,
                  actions/<action-id>/{command,status,progress,started,finished,error}
  firewalls/
    <id>/         name*, rules.json, applied_to.json, labels.json*,
                  actions/<action-id>/...
  ssh_keys/
    <id>/         name*, fingerprint, public_key, labels.json*
  load_balancers/
    <id>/         name*, type, location, public_ipv4, algorithm,
                  services.json, targets.json, labels.json*, metadata.json,
                  actions/<action-id>/...
  networks/
    <id>/         name*, ip_range, subnets.json, routes.json, labels.json*,
                  actions/<action-id>/...
  volumes/
    <id>/         name*, status, size, format, server, location, labels.json*,
                  actions/<action-id>/...
  floating_ips/
    <id>/         name*, description, ip, type, server, location, labels.json*,
                  actions/<action-id>/...
  primary_ips/
    <id>/         name*, ip, type, assignee_id, location, labels.json*,
                  actions/<action-id>/...
  certificates/
    <id>/         name*, type, fingerprint, domain_names, not_valid_before/after,
                  labels.json*, actions/<action-id>/...
  images/
    <id>/         name, type, status, os_flavor, os_version, architecture,
                  disk_size, labels.json*, actions/<action-id>/...
  placement_groups/
    <id>/         name*, type, servers, labels.json*
  isos/
    <id>/         name, description, type, architecture
  locations/
    <id>/         name, country, city, network_zone
  server_types/
    <id>/         name, cores, memory, disk, storage_type, cpu_type, architecture
  dns/
    zones/
      <id>/       name, ttl, status, labels.json, zone_file
        records/
          <name_type>/  name, type, ttl, values
  by-label/
    <selector>/   same structure per resource type (label-supporting types only)
  by-name/
    <type>/
      <name> -> ../../<type>/<id>   (symlinks)
  prometheus/
    servers.prom        Prometheus text format, all server metrics
    load_balancers.prom Prometheus text format, all load balancer metrics
```

`*` — writable

## Writable files

Most resources support writing to `name` (rename) and `labels.json` (update labels):

```bash
echo "new-name" > /tmp/hcloud/servers/12345/name
echo '{"env":"prod","team":"ops"}' > /tmp/hcloud/servers/12345/labels.json
echo '{}' > /tmp/hcloud/servers/12345/labels.json   # clear all labels
```

Server power state via `status`:

```bash
echo off      > /tmp/hcloud/servers/12345/status   # force power off
echo shutdown > /tmp/hcloud/servers/12345/status   # graceful shutdown
echo on       > /tmp/hcloud/servers/12345/status   # power on
```

## Actions

Every resource that supports actions has an `actions/` subdirectory. Actions are sorted newest first and refresh every second.

```bash
# monitor a rebuild
watch cat /tmp/hcloud/servers/12345/actions/98765/progress

# check if an action succeeded
cat /tmp/hcloud/servers/12345/actions/98765/status
```

## Prometheus

The `prometheus/` directory contains `.prom` files in Prometheus text format, compatible with the node_exporter textfile collector:

```bash
node_exporter --collector.textfile.directory=/tmp/hcloud/prometheus
```

Server metrics: `cpu_percent`, `disk_read/write_iops`, `disk_read/write_bandwidth_bytes`, `network_in/out_bandwidth_bytes`, `network_in/out_pps`.

Load balancer metrics: `open_connections`, `connections_per_second`, `requests_per_second`, `bandwidth_in/out_bytes`.

## Navigation

```bash
# by label (any valid Hetzner label selector)
ls /tmp/hcloud/by-label/env=prod/servers/
ls /tmp/hcloud/by-label/env=prod,team=ops/servers/

# by name (symlinks to the ID-based paths)
cat /tmp/hcloud/by-name/servers/my-server/public_ipv4
cat /tmp/hcloud/by-name/firewalls/main-fw/rules.json
```

## Caching

Resources refresh every 10 seconds. Action files refresh every 1 second. Prometheus files refresh every 30 seconds (configurable).

## Flags

```
-debug              print FUSE debug output
-prometheus-ttl     cache TTL for prometheus metrics files (default 30s)
```
