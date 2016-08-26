package collectdvsphere

import (
	"net/url"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/find"
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
