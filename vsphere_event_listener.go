package collectdvsphere

import (
	"net/url"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

// VSphereEventListener connects to a vSphere API and listens for certain
// events, reporting them to a StatsCollector
type VSphereEventListener struct {
	config         VSphereConfig
	statsCollector *StatsCollector
	client         *govmomi.Client
}

// A VSphereConfig provides configuration for a VSphereEventListener
type VSphereConfig struct {
	URL         *url.URL
	Insecure    bool
	ClusterPath string
	BaseVMPath  string
}

// NewVSphereEventListener creates a VSphereEventListener with a given
// configuration. Call Start on the event listener to start listening and
// reporting to the given stats collector.
func NewVSphereEventListener(config VSphereConfig, statsCollector *StatsCollector) *VSphereEventListener {
	return &VSphereEventListener{
		config:         config,
		statsCollector: statsCollector,
	}
}

// Start starts the event listener and begins reporting stats to the
// StatsCollector.
func (l *VSphereEventListener) Start() error {
	l.makeClient()
	l.prefillHosts()
	l.prefillBaseVMs()

	clusterRef, err := l.clusterReference()
	if err != nil {
		return err
	}

	eventManager := event.NewManager(l.client.Client)
	err = eventManager.Events(context.TODO(), []types.ManagedObjectReference{clusterRef}, 25, true, false, l.handleEvents)
	return errors.Wrap(err, "event handling failed")
}

func (l *VSphereEventListener) handleEvents(ee []types.BaseEvent) error {
	for _, baseEvent := range ee {
		// TODO: A lot of the Host and Vm args can be nil, we should handle that
		switch e := baseEvent.(type) {
		case *types.VmPoweredOnEvent:
			l.statsCollector.MarkPowerOnSuccess(e.Host.Name)
		case *types.VmFailedToPowerOnEvent:
			l.statsCollector.MarkPowerOnFailure(e.Host.Name)
		case *types.VmPoweredOffEvent:
			l.statsCollector.MarkPowerOffSuccess(e.Host.Name)
		case *types.VmFailedToPowerOffEvent:
			l.statsCollector.MarkPowerOffFailure(e.Host.Name)
		case *types.VmClonedEvent:
			l.statsCollector.MarkCloneSuccess(e.SourceVm.Name)
		case *types.VmCloneFailedEvent:
			l.statsCollector.MarkCloneFailure(e.Vm.Name)
		}
	}

	return nil
}

func (l *VSphereEventListener) makeClient() (err error) {
	l.client, err = govmomi.NewClient(context.TODO(), l.config.URL, l.config.Insecure)

	return errors.Wrap(err, "failed to create govmomi client")
}

func (l *VSphereEventListener) clusterReference() (types.ManagedObjectReference, error) {
	finder := find.NewFinder(l.client.Client, true)
	cluster, err := finder.ClusterComputeResource(context.TODO(), l.config.ClusterPath)
	if err != nil {
		return types.ManagedObjectReference{}, errors.Wrap(err, "failed to find cluster")
	}

	return cluster.Reference(), nil
}

func (l *VSphereEventListener) prefillHosts() error {
	clusterRef, err := l.clusterReference()
	if err != nil {
		return errors.Wrap(err, "failed to get reference to compute cluster")
	}

	hosts, err := object.NewClusterComputeResource(l.client.Client, clusterRef).Hosts(context.TODO())
	if err != nil {
		return errors.Wrap(err, "failed to list hosts in compute cluster")
	}

	for _, host := range hosts {
		var mhost mo.HostSystem
		err := host.Properties(context.TODO(), host.Reference(), []string{"summary"}, &mhost)
		if err != nil {
			return errors.Wrap(err, "failed to get summary for host")
		}
		name := mhost.Summary.Config.Name
		if name != "" {
			l.statsCollector.ensureHostExists(name)
		}
	}

	return nil
}

func (l *VSphereEventListener) prefillBaseVMs() error {
	if l.config.BaseVMPath == "" {
		// Skip if no base VM path, for backwards compatibility with v1.0.0
		return nil
	}

	finder := find.NewFinder(l.client.Client, true)
	folder, err := finder.Folder(context.TODO(), l.config.BaseVMPath)
	if err != nil {
		return errors.Wrap(err, "failed to find base vm folder")
	}

	children, err := folder.Children(context.TODO())
	if err != nil {
		return errors.Wrap(err, "failed to list children of base vm folder")
	}

	for _, vmRef := range children {
		vm, ok := vmRef.(*object.VirtualMachine)
		if !ok {
			continue
		}

		var mvm mo.VirtualMachine
		err := vm.Properties(context.TODO(), vm.Reference(), []string{"config"}, &mvm)
		if err != nil {
			return errors.Wrap(err, "failed to get config for base VM")
		}
		name := mvm.Config.Name
		if name != "" {
			l.statsCollector.ensureBaseVMExists(name)
		}
	}

	return nil
}
