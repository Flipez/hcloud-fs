package main

import (
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newServersNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Server]{
		cache: newCache(func() ([]*hcloud.Server, error) {
			if selector != "" {
				return client.Server.AllWithOpts(context.Background(),
					hcloud.ServerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Server.All(context.Background())
		}),
		idFn: func(s *hcloud.Server) string { return idStr(s.ID) },
		filesFn: func(s *hcloud.Server) []fileEntry {
			files := []fileEntry{
				textFile("name", s.Name),
				textFile("status", string(s.Status)),
				textFile("created", s.Created.Format(time.RFC3339)),
				jsonFile("labels.json", s.Labels),
				jsonFile("metadata.json", s),
				subDir("actions", newActionsDir(serverActionsFn(client, s))),
			}
			if s.ServerType != nil {
				files = append(files, textFile("server_type", s.ServerType.Name))
			}
			if s.Location != nil {
				files = append(files, textFile("location", s.Location.Name))
			}
			if s.PublicNet.IPv4.IP != nil {
				files = append(files, textFile("public_ipv4", s.PublicNet.IPv4.IP.String()))
			}
			if s.PublicNet.IPv6.IP != nil {
				files = append(files, textFile("public_ipv6", s.PublicNet.IPv6.IP.String()))
			}
			if s.Image != nil {
				files = append(files, textFile("image", s.Image.Name))
			}
			return files
		},
	}
}
