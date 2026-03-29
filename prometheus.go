package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// prometheusNode is the top-level prometheus/ directory.
type prometheusNode struct {
	dirNode
	client *hcloud.Client
	ttl    time.Duration
}

func newPrometheusNode(client *hcloud.Client, ttl time.Duration) fs.InodeEmbedder {
	return &prometheusNode{client: client, ttl: ttl}
}

func (n *prometheusNode) OnAdd(ctx context.Context) {
	addPromFile := func(name string, gen func() (string, error)) {
		child := n.NewPersistentInode(ctx, &prometheusFile{
			ttl:   n.ttl,
			genFn: gen,
		}, fs.StableAttr{})
		n.AddChild(name, child, false)
	}

	addPromFile("servers.prom", func() (string, error) {
		return buildServerMetrics(n.client)
	})
	addPromFile("load_balancers.prom", func() (string, error) {
		return buildLoadBalancerMetrics(n.client)
	})
}

var _ = (fs.NodeOnAdder)((*prometheusNode)(nil))

// prometheusFile is a file whose content is generated and cached for TTL duration.
type prometheusFile struct {
	fs.Inode
	mu      sync.Mutex
	cached  string
	fetched time.Time
	ttl     time.Duration
	genFn   func() (string, error)
}

func (f *prometheusFile) get() string {
	f.mu.Lock()
	if time.Since(f.fetched) < f.ttl {
		cached := f.cached
		f.mu.Unlock()
		return cached
	}
	f.mu.Unlock()

	// Fetch outside the lock so concurrent reads don't block.
	content, err := f.genFn()

	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		return f.cached
	}
	f.cached = content
	f.fetched = time.Now()
	return f.cached
}

func (f *prometheusFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	data := []byte(f.get())
	return &dynamicFileHandle{data: data}, fuse.FOPEN_DIRECT_IO, 0
}

func (f *prometheusFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0444
	return 0
}

var _ = (fs.NodeOpener)((*prometheusFile)(nil))
var _ = (fs.NodeGetattrer)((*prometheusFile)(nil))

// latestMetricVal extracts the last non-empty value from a time series via an index function.
func latestMetricVal(n int, val func(int) string) (float64, bool) {
	for i := n - 1; i >= 0; i-- {
		if v := val(i); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func buildServerMetrics(client *hcloud.Client) (string, error) {
	ctx := context.Background()
	servers, err := client.Server.All(ctx)
	if err != nil {
		return "", err
	}

	type result struct {
		server  *hcloud.Server
		metrics *hcloud.ServerMetrics
		err     error
	}

	results := make([]result, len(servers))
	var wg sync.WaitGroup
	now := time.Now()

	for i, s := range servers {
		wg.Add(1)
		go func(i int, s *hcloud.Server) {
			defer wg.Done()
			m, _, err := client.Server.GetMetrics(ctx, s, hcloud.ServerGetMetricsOpts{
				Types: []hcloud.ServerMetricType{
					hcloud.ServerMetricCPU,
					hcloud.ServerMetricDisk,
					hcloud.ServerMetricNetwork,
				},
				Start: now.Add(-5 * time.Minute),
				End:   now,
				Step:  30,
			})
			results[i] = result{server: s, metrics: m, err: err}
		}(i, s)
	}
	wg.Wait()

	serverMetricDefs := []struct {
		name string
		help string
		key  string
	}{
		{"hcloud_server_cpu_percent", "CPU utilization percentage", "cpu"},
		{"hcloud_server_disk_read_iops", "Disk read IOPS", "disk.0.iops.read"},
		{"hcloud_server_disk_write_iops", "Disk write IOPS", "disk.0.iops.write"},
		{"hcloud_server_disk_read_bandwidth_bytes", "Disk read bandwidth bytes/s", "disk.0.bandwidth.read"},
		{"hcloud_server_disk_write_bandwidth_bytes", "Disk write bandwidth bytes/s", "disk.0.bandwidth.write"},
		{"hcloud_server_network_in_bandwidth_bytes", "Network inbound bandwidth bytes/s", "network.0.bandwidth.in"},
		{"hcloud_server_network_out_bandwidth_bytes", "Network outbound bandwidth bytes/s", "network.0.bandwidth.out"},
		{"hcloud_server_network_in_pps", "Network inbound packets per second", "network.0.pps.in"},
		{"hcloud_server_network_out_pps", "Network outbound packets per second", "network.0.pps.out"},
	}

	var b strings.Builder
	for _, def := range serverMetricDefs {
		fmt.Fprintf(&b, "# HELP %s %s\n", def.name, def.help)
		fmt.Fprintf(&b, "# TYPE %s gauge\n", def.name)
		for _, r := range results {
			if r.err != nil || r.metrics == nil {
				continue
			}
			ts, ok := r.metrics.TimeSeries[def.key]
			if !ok {
				continue
			}
			val, ok := latestMetricVal(len(ts), func(i int) string { return ts[i].Value })
			if !ok {
				continue
			}
			location := ""
			if r.server.Location != nil {
				location = r.server.Location.Name
			}
			fmt.Fprintf(&b, "%s{id=%q,name=%q,location=%q} %g\n",
				def.name, idStr(r.server.ID), r.server.Name, location, val)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func buildLoadBalancerMetrics(client *hcloud.Client) (string, error) {
	ctx := context.Background()
	lbs, err := client.LoadBalancer.All(ctx)
	if err != nil {
		return "", err
	}

	type result struct {
		lb      *hcloud.LoadBalancer
		metrics *hcloud.LoadBalancerMetrics
		err     error
	}

	results := make([]result, len(lbs))
	var wg sync.WaitGroup
	now := time.Now()

	for i, lb := range lbs {
		wg.Add(1)
		go func(i int, lb *hcloud.LoadBalancer) {
			defer wg.Done()
			m, _, err := client.LoadBalancer.GetMetrics(ctx, lb, hcloud.LoadBalancerGetMetricsOpts{
				Types: []hcloud.LoadBalancerMetricType{
					hcloud.LoadBalancerMetricOpenConnections,
					hcloud.LoadBalancerMetricConnectionsPerSecond,
					hcloud.LoadBalancerMetricRequestsPerSecond,
					hcloud.LoadBalancerMetricBandwidth,
				},
				Start: now.Add(-5 * time.Minute),
				End:   now,
				Step:  30,
			})
			results[i] = result{lb: lb, metrics: m, err: err}
		}(i, lb)
	}
	wg.Wait()

	lbMetricDefs := []struct {
		name string
		help string
		key  string
	}{
		{"hcloud_lb_open_connections", "Open connections", "open_connections"},
		{"hcloud_lb_connections_per_second", "Connections per second", "connections_per_second"},
		{"hcloud_lb_requests_per_second", "Requests per second", "requests_per_second"},
		{"hcloud_lb_bandwidth_in_bytes", "Inbound bandwidth bytes/s", "bandwidth.in"},
		{"hcloud_lb_bandwidth_out_bytes", "Outbound bandwidth bytes/s", "bandwidth.out"},
	}

	var b strings.Builder
	for _, def := range lbMetricDefs {
		fmt.Fprintf(&b, "# HELP %s %s\n", def.name, def.help)
		fmt.Fprintf(&b, "# TYPE %s gauge\n", def.name)
		for _, r := range results {
			if r.err != nil || r.metrics == nil {
				continue
			}
			ts, ok := r.metrics.TimeSeries[def.key]
			if !ok {
				continue
			}
			val, ok := latestMetricVal(len(ts), func(i int) string { return ts[i].Value })
			if !ok {
				continue
			}
			location := ""
			if r.lb.Location != nil {
				location = r.lb.Location.Name
			}
			fmt.Fprintf(&b, "%s{id=%q,name=%q,location=%q} %g\n",
				def.name, idStr(r.lb.ID), r.lb.Name, location, val)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}
