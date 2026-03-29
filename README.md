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
    <id>/         name, status, location, public_ipv4, public_ipv6,
                  server_type, image, labels.json, metadata.json
  firewalls/
    <id>/         name, rules.json, applied_to.json, labels.json
  ssh_keys/
    <id>/         name, fingerprint, public_key
  load_balancers/
    <id>/         name, type, location, public_ipv4, algorithm,
                  services.json, targets.json, labels.json
  networks/
    <id>/         name, ip_range, subnets.json, routes.json
  volumes/
    <id>/         name, status, size, format, server, location
  floating_ips/
    <id>/         name, ip, type, server, location
  primary_ips/
    <id>/         name, ip, type, assignee_id, location
  certificates/
    <id>/         name, type, fingerprint, domain_names, not_valid_before/after
  images/
    <id>/         name, type, status, os_flavor, os_version, architecture, disk_size
  placement_groups/
    <id>/         name, type, servers
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
```

## Examples

```bash
# watch a server's status
watch cat /tmp/hcloud/servers/12345/status

# find servers by label
ls /tmp/hcloud/by-label/env=prod/servers/

# navigate by name instead of ID
cat /tmp/hcloud/by-name/servers/my-server/public_ipv4

# combine label and name navigation
cat /tmp/hcloud/by-label/env=prod/servers/$(ls /tmp/hcloud/by-label/env=prod/servers/ | head -1)/name

# get a full zone file
cat /tmp/hcloud/dns/zones/192837/zone_file

# check which firewall rules apply to a server's firewall
cat /tmp/hcloud/firewalls/11111/rules.json
```

## Caching

Data is cached for 10 seconds. File contents always reflect the latest cached data — reading a file never serves stale content from a previous cache window.

## Flags

```
-debug    print FUSE debug output
```
