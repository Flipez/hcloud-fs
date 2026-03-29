package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	ttl     time.Duration
	fetchFn func() ([]T, error)
}

func newCache[T any](fetchFn func() ([]T, error)) *cache[T] {
	return &cache[T]{fetchFn: fetchFn, ttl: cacheTTL}
}

func newCacheWithTTL[T any](ttl time.Duration, fetchFn func() ([]T, error)) *cache[T] {
	return &cache[T]{fetchFn: fetchFn, ttl: ttl}
}

func (c *cache[T]) get() []T {
	c.mu.Lock()
	if time.Since(c.fetched) < c.ttl {
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
	ContentFn func() string     // non-nil: regular file
	WriteFn   func(string) error // non-nil: file is writable; called with trimmed value on close
	DirNode   fs.InodeEmbedder  // non-nil: subdirectory
}

// writableFile is a read-write file. Reads return ContentFn(); writes
// accumulate in the handle and call WriteFn on close.
type writableFile struct {
	fs.Inode
	contentFn func() string
	writeFn   func(string) error
	mu        sync.Mutex
	mtime     time.Time
}

func (f *writableFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &writableFileHandle{
		file: f,
		data: []byte(f.contentFn()),
	}, fuse.FOPEN_DIRECT_IO, 0
}

func (f *writableFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0644
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
	f.mu.Lock()
	mtime := f.mtime
	f.mu.Unlock()
	if mtime.IsZero() {
		mtime = time.Now()
	}
	out.SetTimes(nil, &mtime, nil)
	return 0
}

var _ = (fs.NodeOpener)((*writableFile)(nil))
var _ = (fs.NodeGetattrer)((*writableFile)(nil))

type writableFileHandle struct {
	file  *writableFile
	data  []byte
	dirty bool
}

func (h *writableFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if int(off) >= len(h.data) {
		return fuse.ReadResultData(nil), 0
	}
	end := min(int(off)+len(dest), len(h.data))
	return fuse.ReadResultData(h.data[off:end]), 0
}

func (h *writableFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	end := int(off) + len(data)
	if end > len(h.data) {
		h.data = append(h.data[:off], data...)
	} else {
		copy(h.data[off:], data)
	}
	h.dirty = true
	return uint32(len(data)), 0
}

func (h *writableFileHandle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		if sz < uint64(len(h.data)) {
			h.data = h.data[:sz]
		}
	}
	out.Mode = 0644
	out.Size = uint64(len(h.data))
	return 0
}

func (h *writableFileHandle) Flush(ctx context.Context) syscall.Errno {
	if !h.dirty {
		return 0
	}
	if err := h.file.writeFn(strings.TrimSpace(string(h.data))); err != nil {
		return syscall.EIO
	}
	now := time.Now()
	h.file.mu.Lock()
	h.file.mtime = now
	h.file.mu.Unlock()
	h.dirty = false
	return 0
}

func (h *writableFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0644
	out.Size = uint64(len(h.data))
	h.file.mu.Lock()
	mtime := h.file.mtime
	h.file.mu.Unlock()
	if mtime.IsZero() {
		mtime = time.Now()
	}
	out.SetTimes(nil, &mtime, nil)
	return 0
}

var _ = (fs.FileReader)((*writableFileHandle)(nil))
var _ = (fs.FileWriter)((*writableFileHandle)(nil))
var _ = (fs.FileSetattrer)((*writableFileHandle)(nil))
var _ = (fs.FileFlusher)((*writableFileHandle)(nil))
var _ = (fs.FileGetattrer)((*writableFileHandle)(nil))

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
		mode := uint32(0)
		if f.DirNode != nil {
			mode = syscall.S_IFDIR
		}
		entries[i] = fuse.DirEntry{Name: f.Name, Mode: mode}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *resourceInstanceDir[T]) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	out.SetEntryTimeout(cacheTTL)
	out.SetAttrTimeout(cacheTTL)
	for _, f := range n.getFiles() {
		if f.Name == name {
			if f.DirNode != nil {
				child := n.NewPersistentInode(ctx, f.DirNode, fs.StableAttr{Mode: syscall.S_IFDIR})
				return child, 0
			}
			if f.WriteFn != nil {
				child := n.NewInode(ctx, &writableFile{contentFn: f.ContentFn, writeFn: f.WriteFn}, fs.StableAttr{})
				return child, 0
			}
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

func writableTextFile(name string, contentFn func() string, writeFn func(string) error) fileEntry {
	return fileEntry{Name: name, ContentFn: contentFn, WriteFn: writeFn}
}

// parseLabels parses a JSON object into map[string]string for label updates.
func parseLabels(v string) (map[string]string, error) {
	var labels map[string]string
	if err := json.Unmarshal([]byte(v), &labels); err != nil {
		return nil, err
	}
	return labels, nil
}

func subDir(name string, node fs.InodeEmbedder) fileEntry {
	return fileEntry{Name: name, DirNode: node}
}

func idStr(id int64) string {
	return fmt.Sprintf("%d", id)
}
