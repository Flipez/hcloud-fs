package main

import (
	"context"
	"fmt"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newServerTypesNode(client *hcloud.Client) fs.InodeEmbedder {
	return &resourceDir[*hcloud.ServerType]{
		cache: newCache(func() ([]*hcloud.ServerType, error) {
			return client.ServerType.All(context.Background())
		}),
		idFn: func(st *hcloud.ServerType) string { return idStr(st.ID) },
		filesFn: func(st *hcloud.ServerType) []fileEntry {
			return []fileEntry{
				textFile("name", st.Name),
				textFile("description", st.Description),
				textFile("cores", fmt.Sprintf("%d", st.Cores)),
				textFile("memory", fmt.Sprintf("%.1f", st.Memory)),
				textFile("disk", fmt.Sprintf("%d", st.Disk)),
				textFile("storage_type", string(st.StorageType)),
				textFile("cpu_type", string(st.CPUType)),
				textFile("architecture", string(st.Architecture)),
			}
		},
	}
}
