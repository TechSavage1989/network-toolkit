package diag

import (
	"context"
	"sync"
)

// Discovery overlaps independent checks, with four operations per host and a
// scan-wide ceiling of 64. Ping/TCP sweeps and concurrency=1 keep their existing
// sequential semantics. Metadata never prevents publishing reachability first.
func probeDiscoveryHost(ctx context.Context, address string, options subnetOptions, probes subnetProbes, slots chan struct{}, completed func(), snapshot func(SubnetHost)) SubnetHost {
	tasks := []subnetOptions{}
	add := func(edit func(*subnetOptions)) {
		task := options
		task.icmp = false
		task.ports = nil
		task.identify = false
		task.resolveHostnames = false
		edit(&task)
		tasks = append(tasks, task)
	}
	if options.icmp {
		add(func(o *subnetOptions) { o.icmp = true })
	}
	for _, port := range options.ports {
		port := port
		add(func(o *subnetOptions) { o.ports = []int{port} })
	}
	reachability := len(tasks)
	if options.identify {
		add(func(o *subnetOptions) { o.identify = true })
	}
	if options.resolveHostnames {
		add(func(o *subnetOptions) { o.resolveHostnames = true })
	}
	type piece struct {
		index int
		host  SubnetHost
	}
	jobs := make(chan int)
	results := make(chan piece, len(tasks))
	var wg sync.WaitGroup
	for n := 0; n < min(4, len(tasks)); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					continue
				case slots <- struct{}{}:
				}
				if ctx.Err() != nil {
					<-slots
					continue
				}
				host := probeSubnetHost(ctx, address, tasks[index], probes, completed)
				<-slots
				results <- piece{index, host}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range tasks {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	pieces := make([]*SubnetHost, len(tasks))
	reachDone := 0
	published := false
	aggregate := func(final bool) SubnetHost {
		host := SubnetHost{Address: address, Evidence: []string{}, OpenPorts: []int{}, Completed: final, Enriching: !final, HostnameIdentity: disabledHostname()}
		if options.resolveHostnames {
			host.HostnameIdentity = HostnameIdentity{HostnameStatus: "pending", HostnameNote: "Hostname lookup is in progress."}
		}
		for index, item := range pieces {
			if item == nil {
				if final {
					host.Completed = false
				}
				continue
			}
			if !item.Completed {
				host.Completed = false
			}
			host.Evidence = append(host.Evidence, item.Evidence...)
			host.OpenPorts = append(host.OpenPorts, item.OpenPorts...)
			host.Ports = append(host.Ports, item.Ports...)
			host.Samples = append(host.Samples, item.Samples...)
			if item.ICMPError != "" {
				host.ICMPError = item.ICMPError
			}
			if host.LatencyMs == nil && item.LatencyMs != nil {
				latency := *item.LatencyMs
				host.LatencyMs = &latency
			}
			if tasks[index].identify {
				host.DeviceIdentity = item.DeviceIdentity
			}
			if tasks[index].resolveHostnames {
				host.HostnameIdentity = item.HostnameIdentity
			}
		}
		host.Responded = len(host.Evidence) > 0
		if options.icmp {
			stats := statistics(host.Samples)
			host.Statistics = &stats
		}
		switch {
		case !final:
			host.Status = "scanning"
		case !host.Completed:
			host.Status = "cancelled"
		case host.Responded:
			host.Status = "responded"
		case host.ICMPError != "":
			host.Status = "error"
		default:
			host.Status = "no_response"
			for _, port := range host.Ports {
				if port.State == "error" {
					host.Status = "error"
					break
				}
			}
		}
		if final && host.HostnameStatus == "pending" {
			host.HostnameStatus = "cancelled"
			host.HostnameNote = "Hostname lookup did not finish before cancellation."
		}
		return host
	}
	for result := range results {
		copy := result.host
		pieces[result.index] = &copy
		if result.index < reachability {
			reachDone++
		}
		if !published && reachDone == reachability && reachability < len(tasks) && ctx.Err() == nil {
			published = true
			snapshot(aggregate(false))
		}
	}
	return aggregate(true)
}
