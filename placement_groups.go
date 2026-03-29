package main

import (
	"encoding/json"
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
				writableTextFile("name", func() string { return pg.Name }, func(v string) error {
				_, _, err := client.PlacementGroup.Update(context.Background(), pg, hcloud.PlacementGroupUpdateOpts{Name: v})
				return err
			}),
				textFile("type", string(pg.Type)),
				textFile("created", pg.Created.Format(time.RFC3339)),
				writableTextFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(pg.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.PlacementGroup.Update(context.Background(), pg, hcloud.PlacementGroupUpdateOpts{Labels: labels})
					return err
				}),
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
