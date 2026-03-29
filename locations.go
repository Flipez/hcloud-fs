package main

import (
	"context"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newLocationsNode(client *hcloud.Client) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Location]{
		cache: newCache(func() ([]*hcloud.Location, error) {
			return client.Location.All(context.Background())
		}),
		idFn: func(loc *hcloud.Location) string { return idStr(loc.ID) },
		filesFn: func(loc *hcloud.Location) []fileEntry {
			return []fileEntry{
				textFile("name", loc.Name),
				textFile("description", loc.Description),
				textFile("country", loc.Country),
				textFile("city", loc.City),
				textFile("network_zone", string(loc.NetworkZone)),
			}
		},
	}
}
