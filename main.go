package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func main() {
	debug := flag.Bool("debug", false, "print debug data")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage: hcloud-fs MOUNTPOINT")
	}

	token := os.Getenv("HCLOUD_TOKEN")
	if token == "" {
		log.Fatal("HCLOUD_TOKEN environment variable is required")
	}

	client := hcloud.NewClient(hcloud.WithToken(token))
	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			Options: []string{"nobrowse"},
		},
	}
	opts.Debug = *debug

	server, err := fs.Mount(flag.Arg(0), newRootNode(client), opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		server.Unmount()
		os.Exit(0)
	}()

	log.Printf("Mounted at %s\n", flag.Arg(0))
	server.Wait()
}
