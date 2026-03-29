package main

import (
	"encoding/json"
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newFirewallsNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Firewall]{
		cache: newCache(func() ([]*hcloud.Firewall, error) {
			if selector != "" {
				return client.Firewall.AllWithOpts(context.Background(),
					hcloud.FirewallListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Firewall.All(context.Background())
		}),
		idFn: func(fw *hcloud.Firewall) string { return idStr(fw.ID) },
		filesFn: func(fw *hcloud.Firewall) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return fw.Name }, func(v string) error {
				_, _, err := client.Firewall.Update(context.Background(), fw, hcloud.FirewallUpdateOpts{Name: v})
				return err
			}),
				textFile("created", fw.Created.Format(time.RFC3339)),
				writableJSONFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(fw.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.Firewall.Update(context.Background(), fw, hcloud.FirewallUpdateOpts{Labels: labels})
					return err
				}),
				subDir("actions", newActionsDir(firewallActionsFn(client, fw))),
			}
			if len(fw.Rules) > 0 {
				files = append(files, jsonFile("rules.json", fw.Rules))
			}
			if len(fw.AppliedTo) > 0 {
				files = append(files, jsonFile("applied_to.json", fw.AppliedTo))
			}
			return files
		},
	}
}
