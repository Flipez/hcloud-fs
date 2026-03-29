package main

import (
	"context"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newISOsNode(client *hcloud.Client) fs.InodeEmbedder {
	return &resourceDir[*hcloud.ISO]{
		cache: newCache(func() ([]*hcloud.ISO, error) {
			return client.ISO.All(context.Background())
		}),
		idFn: func(iso *hcloud.ISO) string { return idStr(iso.ID) },
		filesFn: func(iso *hcloud.ISO) []fileEntry {
			files := []fileEntry{
				textFile("name", iso.Name),
				textFile("description", iso.Description),
				textFile("type", string(iso.Type)),
			}
			if iso.Architecture != nil {
				files = append(files, textFile("architecture", string(*iso.Architecture)))
			}
			return files
		},
	}
}
