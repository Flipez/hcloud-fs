package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type dnsNode struct {
	dirNode
	client *hcloud.Client
}

func newDNSNode(client *hcloud.Client) fs.InodeEmbedder {
	return &dnsNode{client: client}
}

func (n *dnsNode) OnAdd(ctx context.Context) {
	zonesDir := n.NewPersistentInode(ctx, newZonesNode(n.client), fs.StableAttr{Mode: syscall.S_IFDIR})
	n.AddChild("zones", zonesDir, false)
}

var _ = (fs.NodeOnAdder)((*dnsNode)(nil))

type zonesNode struct {
	dirNode
	client       *hcloud.Client
	cache        *cache[*hcloud.Zone]
	mu           sync.Mutex
	recordCaches map[int64]*cache[*hcloud.ZoneRRSet]
}

func newZonesNode(client *hcloud.Client) fs.InodeEmbedder {
	return &zonesNode{
		client: client,
		cache: newCache(func() ([]*hcloud.Zone, error) {
			return client.Zone.All(context.Background())
		}),
		recordCaches: make(map[int64]*cache[*hcloud.ZoneRRSet]),
	}
}

func (n *zonesNode) getRecordsCache(z *hcloud.Zone) *cache[*hcloud.ZoneRRSet] {
	n.mu.Lock()
	defer n.mu.Unlock()
	if c, ok := n.recordCaches[z.ID]; ok {
		return c
	}
	c := newCache(func() ([]*hcloud.ZoneRRSet, error) {
		return n.client.Zone.AllRRSets(context.Background(), z)
	})
	n.recordCaches[z.ID] = c
	return c
}

func (n *zonesNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	zones := n.cache.get()
	entries := make([]fuse.DirEntry, len(zones))
	for i, z := range zones {
		entries[i] = fuse.DirEntry{Name: idStr(z.ID), Mode: syscall.S_IFDIR}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *zonesNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	zones := n.cache.get()
	for _, z := range zones {
		if idStr(z.ID) == name {
			child := n.NewPersistentInode(ctx, &zoneNode{
				zone:         z,
				recordsCache: n.getRecordsCache(z),
			}, fs.StableAttr{Mode: syscall.S_IFDIR})
			out.SetEntryTimeout(cacheTTL)
			out.SetAttrTimeout(cacheTTL)
			return child, 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*zonesNode)(nil))
var _ = (fs.NodeLookuper)((*zonesNode)(nil))

type zoneNode struct {
	dirNode
	zone         *hcloud.Zone
	recordsCache *cache[*hcloud.ZoneRRSet]
}

func (n *zoneNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream([]fuse.DirEntry{
		{Name: "name"},
		{Name: "ttl"},
		{Name: "status"},
		{Name: "labels.json"},
		{Name: "records", Mode: syscall.S_IFDIR},
		{Name: "zone_file"},
	}), 0
}

func (n *zoneNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	out.SetEntryTimeout(cacheTTL)
	out.SetAttrTimeout(cacheTTL)

	switch name {
	case "name":
		return n.fileInode(ctx, textFile("name", n.zone.Name)), 0
	case "ttl":
		return n.fileInode(ctx, textFile("ttl", fmt.Sprintf("%d", n.zone.TTL))), 0
	case "status":
		return n.fileInode(ctx, textFile("status", string(n.zone.Status))), 0
	case "labels.json":
		return n.fileInode(ctx, jsonFile("labels.json", n.zone.Labels)), 0
	case "records":
		child := n.NewInode(ctx, &recordsNode{cache: n.recordsCache}, fs.StableAttr{Mode: syscall.S_IFDIR})
		return child, 0
	case "zone_file":
		return n.NewInode(ctx, &dynamicFile{
			contentFn: func() string { return buildZoneFileContent(n.recordsCache, n.zone) },
		}, fs.StableAttr{}), 0
	}
	return nil, syscall.ENOENT
}

func (n *zoneNode) fileInode(ctx context.Context, f fileEntry) *fs.Inode {
	return n.NewInode(ctx, &dynamicFile{contentFn: f.ContentFn}, fs.StableAttr{})
}

var _ = (fs.NodeReaddirer)((*zoneNode)(nil))
var _ = (fs.NodeLookuper)((*zoneNode)(nil))

type recordsNode struct {
	dirNode
	cache *cache[*hcloud.ZoneRRSet]
}

func rrsetDirName(rr *hcloud.ZoneRRSet) string {
	return strings.ReplaceAll(rr.ID, "/", "_")
}

func rrsetTTLStr(rr *hcloud.ZoneRRSet) string {
	if rr.TTL != nil {
		return fmt.Sprintf("%d", *rr.TTL)
	}
	return ""
}

func (n *recordsNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	rrsets := n.cache.get()
	entries := make([]fuse.DirEntry, len(rrsets))
	for i, rr := range rrsets {
		entries[i] = fuse.DirEntry{Name: rrsetDirName(rr), Mode: syscall.S_IFDIR}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *recordsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	rrsets := n.cache.get()
	for _, rr := range rrsets {
		if rrsetDirName(rr) == name {
			values := make([]string, len(rr.Records))
			for i, r := range rr.Records {
				values[i] = r.Value
			}
			files := []fileEntry{
				textFile("name", rr.Name),
				textFile("type", string(rr.Type)),
				textFile("ttl", rrsetTTLStr(rr)),
				textFile("values", strings.Join(values, "\n")),
			}
			child := n.NewInode(ctx, &staticDir{files: files}, fs.StableAttr{Mode: syscall.S_IFDIR})
			return child, 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*recordsNode)(nil))
var _ = (fs.NodeLookuper)((*recordsNode)(nil))

type staticDir struct {
	dirNode
	files []fileEntry
}

func (n *staticDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := make([]fuse.DirEntry, len(n.files))
	for i, f := range n.files {
		entries[i] = fuse.DirEntry{Name: f.Name}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *staticDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for _, f := range n.files {
		if f.Name == name {
			return n.NewInode(ctx, &dynamicFile{contentFn: f.ContentFn}, fs.StableAttr{}), 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*staticDir)(nil))
var _ = (fs.NodeLookuper)((*staticDir)(nil))

func buildZoneFileContent(recordsCache *cache[*hcloud.ZoneRRSet], z *hcloud.Zone) string {
	rrsets := recordsCache.get()

	var b strings.Builder
	fmt.Fprintf(&b, "; Zone: %s\n", z.Name)
	fmt.Fprintf(&b, "; TTL: %d\n\n", z.TTL)

	for _, rr := range rrsets {
		ttl := 0
		if rr.TTL != nil {
			ttl = *rr.TTL
		}
		for _, r := range rr.Records {
			fmt.Fprintf(&b, "%-30s %d IN %-6s %s\n", rr.Name, ttl, rr.Type, r.Value)
		}
	}
	return b.String()
}
