package main

import (
	"encoding/json"
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newNetworksNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Network]{
		cache: newCache(func() ([]*hcloud.Network, error) {
			if selector != "" {
				return client.Network.AllWithOpts(context.Background(),
					hcloud.NetworkListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Network.All(context.Background())
		}),
		idFn: func(net *hcloud.Network) string { return idStr(net.ID) },
		filesFn: func(net *hcloud.Network) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return net.Name }, func(v string) error {
				_, _, err := client.Network.Update(context.Background(), net, hcloud.NetworkUpdateOpts{Name: v})
				return err
			}),
				textFile("ip_range", net.IPRange.String()),
				textFile("created", net.Created.Format(time.RFC3339)),
				writableJSONFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(net.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.Network.Update(context.Background(), net, hcloud.NetworkUpdateOpts{Labels: labels})
					return err
				}),
				subDir("actions", newActionsDir(networkActionsFn(client, net))),
			}
			if len(net.Subnets) > 0 {
				files = append(files, jsonFile("subnets.json", net.Subnets))
			}
			if len(net.Routes) > 0 {
				files = append(files, jsonFile("routes.json", net.Routes))
			}
			return files
		},
	}
}
