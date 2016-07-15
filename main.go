package main

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"collectd.org/api"
	"collectd.org/network"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

type vSphereStatsCollector struct {
	client     *govmomi.Client
	clusterRef types.ManagedObjectReference

	statsMutex      sync.Mutex
	powerOnSuccess  map[string]int64
	powerOnFailure  map[string]int64
	powerOffSuccess map[string]int64
	powerOffFailure map[string]int64
	cloneSuccess    map[string]int64
	cloneFailure    map[string]int64
	eventCount      uint64
}

func (c *vSphereStatsCollector) handleEvent(baseEvent types.BaseEvent) {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()

	switch e := baseEvent.(type) {
	case *types.VmPoweredOnEvent:
		c.ensureHostExists(e.Host.Name)
		c.powerOnSuccess[e.Host.Name]++
	case *types.VmFailedToPowerOnEvent:
		c.ensureHostExists(e.Host.Name)
		c.powerOnFailure[e.Host.Name]++
	case *types.VmPoweredOffEvent:
		c.ensureHostExists(e.Host.Name)
		c.powerOffSuccess[e.Host.Name]++
	case *types.VmFailedToPowerOffEvent:
		c.ensureHostExists(e.Host.Name)
		c.powerOffFailure[e.Host.Name]++
	case *types.VmClonedEvent:
		c.ensureBaseVMExists(e.SourceVm.Name)
		c.cloneSuccess[e.SourceVm.Name]++
	case *types.VmCloneFailedEvent:
		c.ensureBaseVMExists(e.Vm.Name)
		c.cloneFailure[e.Vm.Name]++
	}

	c.eventCount++
}

func (c *vSphereStatsCollector) ensureHostExists(host string) {
	if _, ok := c.powerOnSuccess[host]; !ok {
		c.powerOnSuccess[host] = 0
	}
	if _, ok := c.powerOnFailure[host]; !ok {
		c.powerOnFailure[host] = 0
	}
	if _, ok := c.powerOffSuccess[host]; !ok {
		c.powerOffSuccess[host] = 0
	}
	if _, ok := c.powerOffFailure[host]; !ok {
		c.powerOffFailure[host] = 0
	}
}

func (c *vSphereStatsCollector) ensureBaseVMExists(baseVM string) {
	if _, ok := c.cloneSuccess[baseVM]; !ok {
		c.cloneSuccess[baseVM] = 0
	}
	if _, ok := c.cloneFailure[baseVM]; !ok {
		c.cloneFailure[baseVM] = 0
	}
}

func (c *vSphereStatsCollector) eventListener(ctx context.Context) {
	clusterRefs := []types.ManagedObjectReference{c.clusterRef}
	manager := event.NewManager(c.client.Client)
	err := manager.Events(ctx, clusterRefs, 25, true, false, func(ee []types.BaseEvent) error {
		for _, e := range ee {
			c.handleEvent(e)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("error getting events: %v", err)
	}
}

func (c *vSphereStatsCollector) writeToCollectd(w api.Writer, lastEventCount uint64, interval time.Duration) uint64 {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()

	if c.eventCount > lastEventCount {
		statTime := time.Now()
		for host, stat := range c.powerOnSuccess {
			err := w.Write(makeValueList(host, "power_on_success", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		for host, stat := range c.powerOnFailure {
			err := w.Write(makeValueList(host, "power_on_failure", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		for host, stat := range c.powerOffSuccess {
			err := w.Write(makeValueList(host, "power_off_success", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		for host, stat := range c.powerOffFailure {
			err := w.Write(makeValueList(host, "power_off_failure", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		for host, stat := range c.cloneSuccess {
			err := w.Write(makeValueList(host, "clone_success", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		for host, stat := range c.cloneFailure {
			err := w.Write(makeValueList(host, "clone_failure", statTime, interval, stat))
			if err != nil {
				log.Printf("error sending data to collectd: %v", err)
			}
		}
		log.Printf("wrote stats for %d events", c.eventCount-lastEventCount)
	} else {
		log.Printf("no new events")
	}

	return c.eventCount
}

func makeClient(ctx context.Context) (*govmomi.Client, error) {
	u, err := url.Parse(os.Getenv("VSPHERE_URL"))
	if err != nil {
		return nil, err
	}

	insecure, err := strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))
	if err != nil {
		log.Fatalf("invalid bool in VSPHERE_INSECURE")
		return nil, err
	}

	return govmomi.NewClient(ctx, u, insecure)
}

func findClusterReference(ctx context.Context, client *govmomi.Client) (types.ManagedObjectReference, error) {
	finder := find.NewFinder(client.Client, true)
	cluster, err := finder.ClusterComputeResource(ctx, os.Getenv("VSPHERE_CLUSTER"))
	if err != nil {
		return types.ManagedObjectReference{}, nil
	}

	return cluster.Reference(), nil
}

func makeValueList(host, metric string, statTime time.Time, interval time.Duration, value int64) api.ValueList {
	var valueList api.ValueList
	valueList.Identifier.Host = host
	valueList.Identifier.Plugin = "vsphere"
	valueList.Identifier.Type = "operations"
	valueList.Identifier.TypeInstance = metric
	valueList.Time = statTime
	valueList.Interval = interval
	valueList.Values = []api.Value{api.Derive(value)}

	return valueList
}

func main() {
	ctx := context.Background()
	client, err := makeClient(ctx)
	if err != nil {
		log.Fatalf("couldn't create client: %v", err)
	}

	clusterRef, err := findClusterReference(ctx, client)
	if err != nil {
		log.Fatalf("couldn't find cluster: %v", err)
	}

	statsCollector := &vSphereStatsCollector{
		client:          client,
		clusterRef:      clusterRef,
		powerOnSuccess:  make(map[string]int64, 0),
		powerOnFailure:  make(map[string]int64, 0),
		powerOffSuccess: make(map[string]int64, 0),
		powerOffFailure: make(map[string]int64, 0),
		cloneSuccess:    make(map[string]int64, 0),
		cloneFailure:    make(map[string]int64, 0),
	}

	lastEventCount := statsCollector.eventCount

	statWriter, err := network.Dial(os.Getenv("COLLECTD_HOSTPORT"), network.ClientOptions{
		SecurityLevel: network.Encrypt,
		Username:      os.Getenv("COLLECTD_USERNAME"),
		Password:      os.Getenv("COLLECTD_PASSWORD"),
	})
	if err != nil {
		log.Fatalf("couldn't connect to collectd: %v", err)
	}

	sleepInterval := time.Minute

	log.Printf("starting event listener")

	go statsCollector.eventListener(ctx)

	ticker := time.NewTicker(sleepInterval)
	defer ticker.Stop()

	log.Printf("starting stats updater every %s", sleepInterval.String())
	for range ticker.C {
		lastEventCount = statsCollector.writeToCollectd(statWriter, lastEventCount, sleepInterval)
	}
}
