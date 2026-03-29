package main

import (
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
				textFile("name", net.Name),
				textFile("ip_range", net.IPRange.String()),
				textFile("created", net.Created.Format(time.RFC3339)),
				jsonFile("labels.json", net.Labels),
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
