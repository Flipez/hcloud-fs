package main

import (
	"encoding/json"
	"context"
	"syscall"
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
				writableTextFile("name", func() string { return s.Name }, func(v string) error {
				_, _, err := client.Server.Update(context.Background(), s, hcloud.ServerUpdateOpts{Name: v})
				return err
			}),
				writableTextFile("status",
				func() string { return string(s.Status) },
				func(v string) error {
					switch v {
					case "off":
						_, _, err := client.Server.Poweroff(context.Background(), s)
						return err
					case "running", "on":
						_, _, err := client.Server.Poweron(context.Background(), s)
						return err
					case "shutdown":
						_, _, err := client.Server.Shutdown(context.Background(), s)
						return err
					default:
						return syscall.EINVAL
					}
				}),
				textFile("created", s.Created.Format(time.RFC3339)),
				writableTextFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(s.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.Server.Update(context.Background(), s, hcloud.ServerUpdateOpts{Labels: labels})
					return err
				}),
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
