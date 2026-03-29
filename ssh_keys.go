package main

import (
	"context"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func newSSHKeysNode(client *hcloud.Client, selector string) fs.InodeEmbedder {
	return &resourceDir[*hcloud.SSHKey]{
		cache: newCache(func() ([]*hcloud.SSHKey, error) {
			if selector != "" {
				return client.SSHKey.AllWithOpts(context.Background(),
					hcloud.SSHKeyListOpts{ListOpts: hcloud.ListOpts{LabelSelector: selector}})
			}
			return client.SSHKey.All(context.Background())
		}),
		idFn: func(k *hcloud.SSHKey) string { return idStr(k.ID) },
		filesFn: func(k *hcloud.SSHKey) []fileEntry {
			return []fileEntry{
				textFile("name", k.Name),
				textFile("fingerprint", k.Fingerprint),
				textFile("public_key", k.PublicKey),
				textFile("created", k.Created.Format(time.RFC3339)),
				jsonFile("labels.json", k.Labels),
			}
		},
	}
}
