package main

import (
	"encoding/json"
	"context"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newCertificatesNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.Certificate]{
		cache: newCache(func() ([]*hcloud.Certificate, error) {
			if selector != "" {
				return client.Certificate.AllWithOpts(context.Background(),
					hcloud.CertificateListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.Certificate.All(context.Background())
		}),
		idFn: func(c *hcloud.Certificate) string { return idStr(c.ID) },
		filesFn: func(c *hcloud.Certificate) []fileEntry {
			files := []fileEntry{
				writableTextFile("name", func() string { return c.Name }, func(v string) error {
				_, _, err := client.Certificate.Update(context.Background(), c, hcloud.CertificateUpdateOpts{Name: v})
				return err
			}),
				textFile("type", string(c.Type)),
				textFile("fingerprint", c.Fingerprint),
				textFile("created", c.Created.Format(time.RFC3339)),
				writableTextFile("labels.json",
				func() string {
					data, _ := json.MarshalIndent(c.Labels, "", "  ")
					return string(data) + "\n"
				},
				func(v string) error {
					labels, err := parseLabels(v)
					if err != nil {
						return err
					}
					_, _, err = client.Certificate.Update(context.Background(), c, hcloud.CertificateUpdateOpts{Labels: labels})
					return err
				}),
				subDir("actions", newActionsDir(certificateActionsFn(client, c))),
			}
			if !c.NotValidBefore.IsZero() {
				files = append(files, textFile("not_valid_before", c.NotValidBefore.Format(time.RFC3339)))
			}
			if !c.NotValidAfter.IsZero() {
				files = append(files, textFile("not_valid_after", c.NotValidAfter.Format(time.RFC3339)))
			}
			if len(c.DomainNames) > 0 {
				files = append(files, textFile("domain_names", strings.Join(c.DomainNames, "\n")))
			}
			return files
		},
	}
}
