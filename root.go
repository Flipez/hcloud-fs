package main

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type rootNode struct {
	dirNode
	client *hcloud.Client
}

func newRootNode(client *hcloud.Client) *rootNode {
	return &rootNode{client: client}
}

func (r *rootNode) OnAdd(ctx context.Context) {
	resources := []struct {
		name string
		node fs.InodeEmbedder
	}{
		{"servers", newServersNode(r.client, "")},
		{"firewalls", newFirewallsNode(r.client, "")},
		{"ssh_keys", newSSHKeysNode(r.client, "")},
		{"load_balancers", newLoadBalancersNode(r.client, "")},
		{"networks", newNetworksNode(r.client, "")},
		{"volumes", newVolumesNode(r.client, "")},
		{"floating_ips", newFloatingIPsNode(r.client, "")},
		{"primary_ips", newPrimaryIPsNode(r.client, "")},
		{"certificates", newCertificatesNode(r.client, "")},
		{"images", newImagesNode(r.client, "")},
		{"placement_groups", newPlacementGroupsNode(r.client, "")},
		{"isos", newISOsNode(r.client)},
		{"locations", newLocationsNode(r.client)},
		{"server_types", newServerTypesNode(r.client)},
		{"dns", newDNSNode(r.client)},
		{"by-label", newByLabelNode(r.client)},
		{"by-name", newByNameNode(r.client)},
	}

	for _, res := range resources {
		child := r.NewPersistentInode(ctx, res.node, fs.StableAttr{Mode: syscall.S_IFDIR})
		r.AddChild(res.name, child, false)
	}
}

var _ = (fs.NodeOnAdder)((*rootNode)(nil))
