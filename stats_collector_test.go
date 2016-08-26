package collectdvsphere

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"

	"collectd.org/api"
)

type fakeAPIWriter struct {
	metricsMutex sync.Mutex
	metrics      map[string]api.Value
}

func (w *fakeAPIWriter) readMetric(metric string) api.Value {
	w.metricsMutex.Lock()
	defer w.metricsMutex.Unlock()

	return w.metrics[metric]
}

func (w *fakeAPIWriter) Write(vl api.ValueList) error {
	w.metricsMutex.Lock()
	defer w.metricsMutex.Unlock()

	w.metrics[vl.Identifier.String()] = vl.Values[0]

	return nil
}

func TestStatsCollector(t *testing.T) {
	apiWriter := &fakeAPIWriter{metrics: make(map[string]api.Value)}

	nullLogger := logrus.New()
	nullLogger.Out = ioutil.Discard

	collector := NewStatsCollector(apiWriter, time.Millisecond, nullLogger)
	collector.MarkPowerOnSuccess("on-yes-host")
	collector.MarkPowerOnSuccess("on-yes-host")
	collector.MarkPowerOnFailure("on-no-host")
	collector.MarkPowerOffSuccess("off-yes-host")
	collector.MarkPowerOffFailure("off-no-host")
	collector.MarkCloneSuccess("yes-image")
	collector.MarkCloneFailure("no-image")

	// Sleep for 2 milliseconds to allow the metrics to be written to the
	// fakeAPIWriter.
	time.Sleep(2 * time.Millisecond)

	expectedMetrics := []struct {
		metric string
		value  api.Value
	}{
		{"on-yes-host/vsphere/operations-power_on_success", api.Derive(2)},
		{"on-yes-host/vsphere/operations-power_on_failure", api.Derive(0)},
		{"on-yes-host/vsphere/operations-power_off_success", api.Derive(0)},
		{"on-yes-host/vsphere/operations-power_off_failure", api.Derive(0)},

		{"on-no-host/vsphere/operations-power_on_success", api.Derive(0)},
		{"on-no-host/vsphere/operations-power_on_failure", api.Derive(1)},
		{"on-no-host/vsphere/operations-power_off_success", api.Derive(0)},
		{"on-no-host/vsphere/operations-power_off_failure", api.Derive(0)},

		{"off-yes-host/vsphere/operations-power_on_success", api.Derive(0)},
		{"off-yes-host/vsphere/operations-power_on_failure", api.Derive(0)},
		{"off-yes-host/vsphere/operations-power_off_success", api.Derive(1)},
		{"off-yes-host/vsphere/operations-power_off_failure", api.Derive(0)},

		{"off-no-host/vsphere/operations-power_on_success", api.Derive(0)},
		{"off-no-host/vsphere/operations-power_on_failure", api.Derive(0)},
		{"off-no-host/vsphere/operations-power_off_success", api.Derive(0)},
		{"off-no-host/vsphere/operations-power_off_failure", api.Derive(1)},

		{"yes-image/vsphere/operations-clone_success", api.Derive(1)},
		{"yes-image/vsphere/operations-clone_failure", api.Derive(0)},

		{"no-image/vsphere/operations-clone_success", api.Derive(0)},
		{"no-image/vsphere/operations-clone_failure", api.Derive(1)},
	}

	for _, expected := range expectedMetrics {
		actualValue := apiWriter.readMetric(expected.metric)
		if actualValue != expected.value {
			t.Errorf("expected %s to be %+v, but was %+v", expected.metric, expected.value, actualValue)
		}
	}
}
