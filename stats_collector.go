package collectdvsphere

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	raven "github.com/getsentry/raven-go"
	"github.com/pkg/errors"

	"collectd.org/api"
)

// A StatsCollector stores various stats that are sent to it and allows you to fetch metrics based on them.
type StatsCollector struct {
	writer   api.Writer
	interval time.Duration
	logger   logrus.FieldLogger

	mutex sync.Mutex

	// Whether any new events have been received since the last write.
	newEvents bool

	// The time of the last time the events were written to the api.Writer.
	lastWrite time.Time

	// Host stats
	powerOnSuccess  map[string]int64
	powerOnFailure  map[string]int64
	powerOffSuccess map[string]int64
	powerOffFailure map[string]int64

	// Base VM stats
	cloneSuccess map[string]int64
	cloneFailure map[string]int64
}

// NewStatsCollector returns a new StatsCollector with no stats, which writes
// its stats to the given api.Writer every interval.
func NewStatsCollector(writer api.Writer, interval time.Duration, logger logrus.FieldLogger) *StatsCollector {
	collector := &StatsCollector{
		writer:          writer,
		interval:        interval,
		powerOnSuccess:  make(map[string]int64),
		powerOnFailure:  make(map[string]int64),
		powerOffSuccess: make(map[string]int64),
		powerOffFailure: make(map[string]int64),
		cloneSuccess:    make(map[string]int64),
		cloneFailure:    make(map[string]int64),
	}

	go func(collector *StatsCollector) {
		ticker := time.NewTicker(collector.interval)
		for range ticker.C {
			err := collector.writeToCollectd()
			if err != nil {
				collector.logger.WithField("err", err).Info("failed writing to collectd")
				raven.CaptureError(err, nil)
			}
		}
	}(collector)

	return collector
}

// MarkPowerOnSuccess increases the number of successful VM power-on events on a
// host with a given hostname.
func (c *StatsCollector) MarkPowerOnSuccess(hostname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureHostExists(hostname)
	c.powerOnSuccess[hostname]++
	c.newEvents = true
}

// MarkPowerOnFailure increases the number of failed VM power-on events on a
// host with a given hostname.
func (c *StatsCollector) MarkPowerOnFailure(hostname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureHostExists(hostname)
	c.powerOnFailure[hostname]++
	c.newEvents = true
}

// MarkPowerOffSuccess increases the number of successful VM power-off events
// on a host with a given hostname.
func (c *StatsCollector) MarkPowerOffSuccess(hostname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureHostExists(hostname)
	c.powerOffSuccess[hostname]++
	c.newEvents = true
}

// MarkPowerOffFailure increases the number of failed VM power-off events on a
// host with a given hostname.
func (c *StatsCollector) MarkPowerOffFailure(hostname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureHostExists(hostname)
	c.powerOffFailure[hostname]++
	c.newEvents = true
}

// MarkCloneSuccess increases the number of successful clones of a base VM with
// a given name.
func (c *StatsCollector) MarkCloneSuccess(baseVMName string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureBaseVMExists(baseVMName)
	c.cloneSuccess[baseVMName]++
	c.newEvents = true
}

// MarkCloneFailure increases the number of failed clones of a base VM with a
// given name.
func (c *StatsCollector) MarkCloneFailure(baseVMName string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ensureBaseVMExists(baseVMName)
	c.cloneFailure[baseVMName]++
	c.newEvents = true
}

func (c *StatsCollector) writeToCollectd() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.newEvents {
		return nil
	}

	statTime := time.Now()
	c.lastWrite = statTime

	events := 0

	for host, stat := range c.powerOnSuccess {
		events++
		err := c.writer.Write(c.makeValueList(host, "power_on_success", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write power_on_success metric")
		}
	}
	for host, stat := range c.powerOnFailure {
		events++
		err := c.writer.Write(c.makeValueList(host, "power_on_failure", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write power_on_failure metric")
		}
	}
	for host, stat := range c.powerOffSuccess {
		events++
		err := c.writer.Write(c.makeValueList(host, "power_off_success", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write power_off_success metric")
		}
	}
	for host, stat := range c.powerOffFailure {
		events++
		err := c.writer.Write(c.makeValueList(host, "power_off_failure", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write power_off_failure metric")
		}
	}
	for baseVM, stat := range c.cloneSuccess {
		events++
		err := c.writer.Write(c.makeValueList(baseVM, "clone_success", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write clone_success metric")
		}
	}
	for baseVM, stat := range c.cloneFailure {
		events++
		err := c.writer.Write(c.makeValueList(baseVM, "clone_failure", statTime, stat))
		if err != nil {
			return errors.Wrap(err, "failed to write clone_failure metric")
		}
	}

	c.logger.WithField("event_count", events).Info("sent metrics to collectd")

	return nil
}

func (c *StatsCollector) makeValueList(host, metric string, statTime time.Time, value int64) api.ValueList {
	var valueList api.ValueList
	valueList.Identifier.Host = host
	valueList.Identifier.Plugin = "vsphere"
	valueList.Identifier.Type = "operations"
	valueList.Identifier.TypeInstance = metric
	valueList.Time = statTime
	valueList.Interval = c.interval
	valueList.Values = []api.Value{api.Derive(value)}

	return valueList
}

func (c *StatsCollector) ensureHostExists(hostname string) {
	if _, ok := c.powerOnSuccess[hostname]; !ok {
		c.powerOnSuccess[hostname] = 0
	}
	if _, ok := c.powerOnFailure[hostname]; !ok {
		c.powerOnFailure[hostname] = 0
	}
	if _, ok := c.powerOffSuccess[hostname]; !ok {
		c.powerOffSuccess[hostname] = 0
	}
	if _, ok := c.powerOffFailure[hostname]; !ok {
		c.powerOffFailure[hostname] = 0
	}
}

func (c *StatsCollector) ensureBaseVMExists(baseVMName string) {
	if _, ok := c.cloneSuccess[baseVMName]; !ok {
		c.cloneSuccess[baseVMName] = 0
	}
	if _, ok := c.cloneFailure[baseVMName]; !ok {
		c.cloneFailure[baseVMName] = 0
	}
}
