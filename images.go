package main

import (
	"encoding/json"
	"context"
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newImagesNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Image]{
		cache: newCache(func() ([]*hcloud.Image, error) {
			if selector != "" {
				return client.Image.AllWithOpts(context.Background(),
					hcloud.ImageListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Image.All(context.Background())
		}),
		idFn: func(img *hcloud.Image) string { return idStr(img.ID) },
		filesFn: func(img *hcloud.Image) []fileEntry {
			return []fileEntry{
				textFile("name", img.Name),
				textFile("description", img.Description),
				textFile("type", string(img.Type)),
				textFile("status", string(img.Status)),
				textFile("os_flavor", img.OSFlavor),
				textFile("os_version", img.OSVersion),
				textFile("architecture", string(img.Architecture)),
				textFile("disk_size", fmt.Sprintf("%.1f", img.DiskSize)),
				textFile("created", img.Created.Format(time.RFC3339)),
				writableJSONFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(img.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.Image.Update(context.Background(), img, hcloud.ImageUpdateOpts{Labels: labels})
					return err
				}),
				subDir("actions", newActionsDir(imageActionsFn(client, img))),
			}
		},
	}
}
