package main

import (
	"context"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newPlacementGroupsNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.PlacementGroup]{
		cache: newCache(func() ([]*hcloud.PlacementGroup, error) {
			if selector != "" {
				return client.PlacementGroup.AllWithOpts(context.Background(),
					hcloud.PlacementGroupListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.PlacementGroup.All(context.Background())
		}),
		idFn: func(pg *hcloud.PlacementGroup) string { return idStr(pg.ID) },
		filesFn: func(pg *hcloud.PlacementGroup) []fileEntry {
			files := []fileEntry{
				textFile("name", pg.Name),
				textFile("type", string(pg.Type)),
				textFile("created", pg.Created.Format(time.RFC3339)),
				jsonFile("labels.json", pg.Labels),
			}
			if len(pg.Servers) > 0 {
				ids := make([]string, len(pg.Servers))
				for i, id := range pg.Servers {
					ids[i] = idStr(id)
				}
				files = append(files, textFile("servers", strings.Join(ids, "\n")))
			}
			return files
		},
	}
}
