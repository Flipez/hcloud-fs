package main

import (
	"encoding/json"
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newFloatingIPsNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.FloatingIP]{
		cache: newCache(func() ([]*hcloud.FloatingIP, error) {
			if selector != "" {
				return client.FloatingIP.AllWithOpts(context.Background(),
					hcloud.FloatingIPListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.FloatingIP.All(context.Background())
		}),
		idFn: func(fip *hcloud.FloatingIP) string { return idStr(fip.ID) },
		filesFn: func(fip *hcloud.FloatingIP) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return fip.Name }, func(v string) error {
				_, _, err := client.FloatingIP.Update(context.Background(), fip, hcloud.FloatingIPUpdateOpts{Name: v})
				return err
			}),
				textFile("description", fip.Description),
				textFile("ip", fip.IP.String()),
				textFile("type", string(fip.Type)),
				textFile("created", fip.Created.Format(time.RFC3339)),
				writableJSONFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(fip.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.FloatingIP.Update(context.Background(), fip, hcloud.FloatingIPUpdateOpts{Labels: labels})
					return err
				}),
				subDir("actions", newActionsDir(floatingIPActionsFn(client, fip))),
			}
			if fip.Server != nil {
				files = append(files, textFile("server", idStr(fip.Server.ID)))
			}
			if fip.HomeLocation != nil {
				files = append(files, textFile("location", fip.HomeLocation.Name))
			}
			return files
		},
	}
}
