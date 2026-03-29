package main

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type actionsDir struct {
	dirNode
	cache *cache[*hcloud.Action]
}

func newActionsDir(fetchFn func() ([]*hcloud.Action, error)) *actionsDir {
	return &actionsDir{
		cache: newCacheWithTTL(time.Second, fetchFn),
	}
}

func (n *actionsDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	actions := n.cache.get()
	entries := make([]fuse.DirEntry, len(actions))
	for i, a := range actions {
		entries[i] = fuse.DirEntry{Name: idStr(a.ID), Mode: syscall.S_IFDIR}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *actionsDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for _, a := range n.cache.get() {
		if idStr(a.ID) == name {
			child := n.NewInode(ctx, &actionInstanceDir{
				cache: n.cache,
				id:    name,
			}, fs.StableAttr{Mode: syscall.S_IFDIR})
			return child, 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*actionsDir)(nil))
var _ = (fs.NodeLookuper)((*actionsDir)(nil))

// actionInstanceDir re-reads from the cache on every file access so that
// progress and status always reflect the latest data.
type actionInstanceDir struct {
	dirNode
	cache *cache[*hcloud.Action]
	id    string
}

func (n *actionInstanceDir) getAction() *hcloud.Action {
	for _, a := range n.cache.get() {
		if idStr(a.ID) == n.id {
			return a
		}
	}
	return nil
}

func (n *actionInstanceDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	a := n.getAction()
	if a == nil {
		return fs.NewListDirStream(nil), 0
	}
	names := []string{"command", "status", "progress", "started"}
	if !a.Finished.IsZero() {
		names = append(names, "finished")
	}
	if a.ErrorCode != "" {
		names = append(names, "error")
	}
	entries := make([]fuse.DirEntry, len(names))
	for i, name := range names {
		entries[i] = fuse.DirEntry{Name: name}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *actionInstanceDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	contentFn := func() string {
		a := n.getAction()
		if a == nil {
			return ""
		}
		switch name {
		case "command":
			return a.Command + "\n"
		case "status":
			return string(a.Status) + "\n"
		case "progress":
			return fmt.Sprintf("%d\n", a.Progress)
		case "started":
			return a.Started.Format(time.RFC3339) + "\n"
		case "finished":
			if !a.Finished.IsZero() {
				return a.Finished.Format(time.RFC3339) + "\n"
			}
		case "error":
			if a.ErrorCode != "" {
				return fmt.Sprintf("%s: %s\n", a.ErrorCode, a.ErrorMessage)
			}
		}
		return ""
	}

	// Validate the name exists before returning a node.
	a := n.getAction()
	if a == nil {
		return nil, syscall.ENOENT
	}
	switch name {
	case "command", "status", "progress", "started":
	case "finished":
		if a.Finished.IsZero() {
			return nil, syscall.ENOENT
		}
	case "error":
		if a.ErrorCode == "" {
			return nil, syscall.ENOENT
		}
	default:
		return nil, syscall.ENOENT
	}

	child := n.NewInode(ctx, &dynamicFile{contentFn: contentFn}, fs.StableAttr{})
	return child, 0
}

var _ = (fs.NodeReaddirer)((*actionInstanceDir)(nil))
var _ = (fs.NodeLookuper)((*actionInstanceDir)(nil))

// actionsFetchFn helpers for each resource type with an Action client.

func serverActionsFn(client *hcloud.Client, s *hcloud.Server) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Server.Action.AllFor(context.Background(), s, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func volumeActionsFn(client *hcloud.Client, v *hcloud.Volume) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Volume.Action.AllFor(context.Background(), v, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func networkActionsFn(client *hcloud.Client, net *hcloud.Network) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Network.Action.AllFor(context.Background(), net, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func loadBalancerActionsFn(client *hcloud.Client, lb *hcloud.LoadBalancer) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.LoadBalancer.Action.AllFor(context.Background(), lb, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func floatingIPActionsFn(client *hcloud.Client, fip *hcloud.FloatingIP) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.FloatingIP.Action.AllFor(context.Background(), fip, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func primaryIPActionsFn(client *hcloud.Client, pip *hcloud.PrimaryIP) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.PrimaryIP.Action.AllFor(context.Background(), pip, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func imageActionsFn(client *hcloud.Client, img *hcloud.Image) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Image.Action.AllFor(context.Background(), img, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func certificateActionsFn(client *hcloud.Client, c *hcloud.Certificate) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Certificate.Action.AllFor(context.Background(), c, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}

func firewallActionsFn(client *hcloud.Client, fw *hcloud.Firewall) func() ([]*hcloud.Action, error) {
	return func() ([]*hcloud.Action, error) {
		return client.Firewall.Action.AllFor(context.Background(), fw, hcloud.ActionListOpts{Sort: []string{"started:desc"}})
	}
}
