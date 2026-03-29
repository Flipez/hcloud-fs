package main

import (
	"encoding/json"
	"context"
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newPrimaryIPsNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.PrimaryIP]{
		cache: newCache(func() ([]*hcloud.PrimaryIP, error) {
			if selector != "" {
				return client.PrimaryIP.AllWithOpts(context.Background(),
					hcloud.PrimaryIPListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.PrimaryIP.All(context.Background())
		}),
		idFn: func(pip *hcloud.PrimaryIP) string { return idStr(pip.ID) },
		filesFn: func(pip *hcloud.PrimaryIP) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return pip.Name }, func(v string) error {
				_, _, err := client.PrimaryIP.Update(context.Background(), pip, hcloud.PrimaryIPUpdateOpts{Name: v})
				return err
			}),
				textFile("ip", pip.IP.String()),
				textFile("type", string(pip.Type)),
				textFile("created", pip.Created.Format(time.RFC3339)),
				writableTextFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(pip.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.PrimaryIP.Update(context.Background(), pip, hcloud.PrimaryIPUpdateOpts{Labels: &labels})
					return err
				}),
				subDir("actions", newActionsDir(primaryIPActionsFn(client, pip))),
			}
			if pip.AssigneeID != 0 {
				files = append(files, textFile("assignee_id", fmt.Sprintf("%d", pip.AssigneeID)))
			}
			if pip.Location != nil {
				files = append(files, textFile("location", pip.Location.Name))
			}
			return files
		},
	}
}
