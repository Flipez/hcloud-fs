package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newVolumesNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Volume]{
		cache: newCache(func() ([]*hcloud.Volume, error) {
			if selector != "" {
				return client.Volume.AllWithOpts(context.Background(),
					hcloud.VolumeListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Volume.All(context.Background())
		}),
		idFn: func(v *hcloud.Volume) string { return idStr(v.ID) },
		filesFn: func(v *hcloud.Volume) []fileEntry {
			files := []fileEntry{
				textFile("name", v.Name),
				textFile("status", string(v.Status)),
				textFile("size", fmt.Sprintf("%d", v.Size)),
				textFile("linux_device", v.LinuxDevice),
				textFile("created", v.Created.Format(time.RFC3339)),
				jsonFile("labels.json", v.Labels),
				subDir("actions", newActionsDir(volumeActionsFn(client, v))),
			}
			if v.Format != nil {
				files = append(files, textFile("format", *v.Format))
			}
			if v.Server != nil {
				files = append(files, textFile("server", idStr(v.Server.ID)))
			}
			if v.Location != nil {
				files = append(files, textFile("location", v.Location.Name))
			}
			return files
		},
	}
}
