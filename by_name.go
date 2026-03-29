package main

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type namedResource struct {
	id   string
	name string
}

var byNameResources = map[string]func(*hcloud.Client) *cache[namedResource]{
	"servers": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Server.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"firewalls": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Firewall.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"ssh_keys": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.SSHKey.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"load_balancers": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.LoadBalancer.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"networks": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Network.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"volumes": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Volume.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"floating_ips": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.FloatingIP.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"primary_ips": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.PrimaryIP.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"certificates": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Certificate.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"images": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Image.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, 0, len(items))
			for _, v := range items {
				if v.Name == "" {
					continue // skip unnamed images (backups without a name)
				}
				r = append(r, namedResource{id: idStr(v.ID), name: v.Name})
			}
			return r, nil
		})
	},
	"placement_groups": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.PlacementGroup.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"isos": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.ISO.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"locations": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.Location.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
	"server_types": func(c *hcloud.Client) *cache[namedResource] {
		return newCache(func() ([]namedResource, error) {
			items, err := c.ServerType.All(context.Background())
			if err != nil {
				return nil, err
			}
			r := make([]namedResource, len(items))
			for i, v := range items {
				r[i] = namedResource{id: idStr(v.ID), name: v.Name}
			}
			return r, nil
		})
	},
}

type byNameNode struct {
	dirNode
	client *hcloud.Client
}

func newByNameNode(client *hcloud.Client) fs.InodeEmbedder {
	return &byNameNode{client: client}
}

func (n *byNameNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := make([]fuse.DirEntry, 0, len(byNameResources))
	for name := range byNameResources {
		entries = append(entries, fuse.DirEntry{Name: name, Mode: syscall.S_IFDIR})
	}
	return fs.NewListDirStream(entries), 0
}

func (n *byNameNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	makeCache, ok := byNameResources[name]
	if !ok {
		return nil, syscall.ENOENT
	}
	child := n.NewPersistentInode(ctx, &byNameTypeNode{
		typeName: name,
		cache:    makeCache(n.client),
	}, fs.StableAttr{Mode: syscall.S_IFDIR})
	out.SetEntryTimeout(cacheTTL)
	out.SetAttrTimeout(cacheTTL)
	return child, 0
}

var _ = (fs.NodeReaddirer)((*byNameNode)(nil))
var _ = (fs.NodeLookuper)((*byNameNode)(nil))

// byNameTypeNode lists resources by name for one resource type.
// Each entry is a symlink: <name> -> ../../<typeName>/<id>
type byNameTypeNode struct {
	dirNode
	typeName string
	cache    *cache[namedResource]
}

func (n *byNameTypeNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	items := n.cache.get()
	entries := make([]fuse.DirEntry, len(items))
	for i, item := range items {
		entries[i] = fuse.DirEntry{Name: item.name, Mode: syscall.S_IFLNK}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *byNameTypeNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for _, item := range n.cache.get() {
		if item.name == name {
			target := fmt.Sprintf("../../%s/%s", n.typeName, item.id)
			child := n.NewInode(ctx, &fs.MemSymlink{Data: []byte(target)}, fs.StableAttr{Mode: syscall.S_IFLNK})
			return child, 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*byNameTypeNode)(nil))
var _ = (fs.NodeLookuper)((*byNameTypeNode)(nil))
