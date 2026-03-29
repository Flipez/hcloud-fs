package main

import (
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
				textFile("name", c.Name),
				textFile("type", string(c.Type)),
				textFile("fingerprint", c.Fingerprint),
				textFile("created", c.Created.Format(time.RFC3339)),
				jsonFile("labels.json", c.Labels),
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
