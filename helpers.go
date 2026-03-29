package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const cacheTTL = 10 * time.Second

type dirNode struct {
	fs.Inode
}

func (n *dirNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

var _ = (fs.NodeGetattrer)((*dirNode)(nil))

type dynamicFile struct {
	fs.Inode
	contentFn func() string
}

func (f *dynamicFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	content := f.contentFn()
	return &dynamicFileHandle{data: []byte(content)}, fuse.FOPEN_DIRECT_IO, 0
}

func (f *dynamicFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0444
	return 0
}

var _ = (fs.NodeOpener)((*dynamicFile)(nil))
var _ = (fs.NodeGetattrer)((*dynamicFile)(nil))

type dynamicFileHandle struct {
	data []byte
}

func (h *dynamicFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if int(off) >= len(h.data) {
		return fuse.ReadResultData(nil), 0
	}
	end := min(int(off)+len(dest), len(h.data))
	return fuse.ReadResultData(h.data[off:end]), 0
}

func (h *dynamicFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0444
	out.Size = uint64(len(h.data))
	return 0
}

var _ = (fs.FileReader)((*dynamicFileHandle)(nil))
var _ = (fs.FileGetattrer)((*dynamicFileHandle)(nil))

type cache[T any] struct {
	mu      sync.Mutex
	data    []T
	fetched time.Time
	fetchFn func() ([]T, error)
}

func newCache[T any](fetchFn func() ([]T, error)) *cache[T] {
	return &cache[T]{fetchFn: fetchFn}
}

func (c *cache[T]) get() []T {
	c.mu.Lock()
	if time.Since(c.fetched) < cacheTTL {
		data := c.data
		c.mu.Unlock()
		return data
	}
	data, err := c.fetchFn()
	if err != nil {
		stale := c.data
		c.mu.Unlock()
		return stale
	}
	c.data = data
	c.fetched = time.Now()
	c.mu.Unlock()
	return data
}

func (c *cache[T]) find(idFn func(T) string, id string) (T, bool) {
	for _, item := range c.get() {
		if idFn(item) == id {
			return item, true
		}
	}
	var zero T
	return zero, false
}

type fileEntry struct {
	Name      string
	ContentFn func() string
}

type resourceDir[T any] struct {
	dirNode
	cache   *cache[T]
	idFn    func(T) string
	filesFn func(T) []fileEntry
}

func (n *resourceDir[T]) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	items := n.cache.get()
	entries := make([]fuse.DirEntry, len(items))
	for i, item := range items {
		entries[i] = fuse.DirEntry{
			Name: n.idFn(item),
			Mode: syscall.S_IFDIR,
		}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *resourceDir[T]) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	_, found := n.cache.find(n.idFn, name)
	if !found {
		return nil, syscall.ENOENT
	}
	child := n.NewPersistentInode(ctx, &resourceInstanceDir[T]{
		cache:   n.cache,
		id:      name,
		idFn:    n.idFn,
		filesFn: n.filesFn,
	}, fs.StableAttr{Mode: syscall.S_IFDIR})
	return child, 0
}

var _ = (fs.NodeReaddirer)((*resourceDir[any])(nil))
var _ = (fs.NodeLookuper)((*resourceDir[any])(nil))

type resourceInstanceDir[T any] struct {
	dirNode
	cache   *cache[T]
	id      string
	idFn    func(T) string
	filesFn func(T) []fileEntry
}

func (n *resourceInstanceDir[T]) getFiles() []fileEntry {
	item, found := n.cache.find(n.idFn, n.id)
	if !found {
		return nil
	}
	return n.filesFn(item)
}

func (n *resourceInstanceDir[T]) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	files := n.getFiles()
	entries := make([]fuse.DirEntry, len(files))
	for i, f := range files {
		entries[i] = fuse.DirEntry{Name: f.Name}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *resourceInstanceDir[T]) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for _, f := range n.getFiles() {
		if f.Name == name {
			child := n.NewInode(ctx, &dynamicFile{contentFn: f.ContentFn}, fs.StableAttr{})
			return child, 0
		}
	}
	return nil, syscall.ENOENT
}

var _ = (fs.NodeReaddirer)((*resourceInstanceDir[any])(nil))
var _ = (fs.NodeLookuper)((*resourceInstanceDir[any])(nil))

func textFile(name string, content string) fileEntry {
	return fileEntry{Name: name, ContentFn: func() string { return content + "\n" }}
}

func jsonFile(name string, v any) fileEntry {
	return fileEntry{Name: name, ContentFn: func() string {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "{}\n"
		}
		return string(data) + "\n"
	}}
}

func idStr(id int64) string {
	return fmt.Sprintf("%d", id)
}
