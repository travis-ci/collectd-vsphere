package collectdvsphere

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"collectd.org/network"
)

func main() {
	statWriter, err := network.Dial(os.Getenv("COLLECTD_HOSTPORT"), network.ClientOptions{
		SecurityLevel: network.Encrypt,
		Username:      os.Getenv("COLLECTD_USERNAME"),
		Password:      os.Getenv("COLLECTD_PASSWORD"),
	})
	if err != nil {
		log.Fatalf("couldn't connect to collectd: %v", err)
	}

	statsCollector := NewStatsCollector(statWriter, time.Minute)

	u, err := url.Parse(os.Getenv("VSPHERE_URL"))
	if err != nil {
		log.Fatalf("couldn't parse VSPHERE_URL: %v", err)
	}
	insecure, err := strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))
	if err != nil {
		log.Fatalf("invalid bool in VSPHERE_INSECURE: %v", err)
	}
	eventListener := NewVSphereEventListener(VSphereConfig{
		URL:         u,
		Insecure:    insecure,
		ClusterPath: os.Getenv("VSPHERE_CLUSTER"),
	}, statsCollector)

	eventListener.Start()
}
