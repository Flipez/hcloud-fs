package main

import (
	"encoding/json"
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newLoadBalancersNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.LoadBalancer]{
		cache: newCache(func() ([]*hcloud.LoadBalancer, error) {
			if selector != "" {
				return client.LoadBalancer.AllWithOpts(context.Background(),
					hcloud.LoadBalancerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.LoadBalancer.All(context.Background())
		}),
		idFn: func(lb *hcloud.LoadBalancer) string { return idStr(lb.ID) },
		filesFn: func(lb *hcloud.LoadBalancer) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return lb.Name }, func(v string) error {
				_, _, err := client.LoadBalancer.Update(context.Background(), lb, hcloud.LoadBalancerUpdateOpts{Name: v})
				return err
			}),
				textFile("created", lb.Created.Format(time.RFC3339)),
				textFile("algorithm", string(lb.Algorithm.Type)),
				writableTextFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(lb.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.LoadBalancer.Update(context.Background(), lb, hcloud.LoadBalancerUpdateOpts{Labels: labels})
					return err
				}),
				jsonFile("metadata.json", lb),
				subDir("actions", newActionsDir(loadBalancerActionsFn(client, lb))),
			}
			if lb.LoadBalancerType != nil {
				files = append(files, textFile("type", lb.LoadBalancerType.Name))
			}
			if lb.Location != nil {
				files = append(files, textFile("location", lb.Location.Name))
			}
			if lb.PublicNet.IPv4.IP != nil {
				files = append(files, textFile("public_ipv4", lb.PublicNet.IPv4.IP.String()))
			}
			if lb.PublicNet.IPv6.IP != nil {
				files = append(files, textFile("public_ipv6", lb.PublicNet.IPv6.IP.String()))
			}
			if len(lb.Services) > 0 {
				files = append(files, jsonFile("services.json", lb.Services))
			}
			if len(lb.Targets) > 0 {
				files = append(files, jsonFile("targets.json", lb.Targets))
			}
			return files
		},
	}
}
