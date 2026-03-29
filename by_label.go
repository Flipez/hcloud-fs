package main

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type resourceFactory func(client *hcloud.Client, selector string) fs.InodeEmbedder

var labelFilterableResources = map[string]resourceFactory{
	"servers":          newServersNode,
	"firewalls":        newFirewallsNode,
	"ssh_keys":         newSSHKeysNode,
	"load_balancers":   newLoadBalancersNode,
	"networks":         newNetworksNode,
	"volumes":          newVolumesNode,
	"floating_ips":     newFloatingIPsNode,
	"primary_ips":      newPrimaryIPsNode,
	"certificates":     newCertificatesNode,
	"images":           newImagesNode,
	"placement_groups": newPlacementGroupsNode,
}

type byLabelNode struct {
	dirNode
	client *hcloud.Client
}

func newByLabelNode(client *hcloud.Client) fs.InodeEmbedder {
	return &byLabelNode{client: client}
}

func (n *byLabelNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream(nil), 0
}

func (n *byLabelNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child := n.NewPersistentInode(ctx, &labelFilterNode{
		client:   n.client,
		selector: name,
	}, fs.StableAttr{Mode: syscall.S_IFDIR})
	out.SetEntryTimeout(cacheTTL)
	out.SetAttrTimeout(cacheTTL)
	return child, 0
}

var _ = (fs.NodeReaddirer)((*byLabelNode)(nil))
var _ = (fs.NodeLookuper)((*byLabelNode)(nil))

type labelFilterNode struct {
	dirNode
	client   *hcloud.Client
	selector string
}

func (n *labelFilterNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := make([]fuse.DirEntry, 0, len(labelFilterableResources))
	for name := range labelFilterableResources {
		entries = append(entries, fuse.DirEntry{Name: name, Mode: syscall.S_IFDIR})
	}
	return fs.NewListDirStream(entries), 0
}

func (n *labelFilterNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	factory, ok := labelFilterableResources[name]
	if !ok {
		return nil, syscall.ENOENT
	}
	child := n.NewPersistentInode(ctx, factory(n.client, n.selector), fs.StableAttr{Mode: syscall.S_IFDIR})
	out.SetEntryTimeout(cacheTTL)
	out.SetAttrTimeout(cacheTTL)
	return child, 0
}

var _ = (fs.NodeReaddirer)((*labelFilterNode)(nil))
var _ = (fs.NodeLookuper)((*labelFilterNode)(nil))
