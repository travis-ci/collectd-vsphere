package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"collectd.org/network"
	collectdvsphere "github.com/travis-ci/collectd-vsphere"
	"github.com/urfave/cli"
)

var (
	// VersionString is the git describe version set at build time
	VersionString = "?"
	// RevisionString is the git revision set at build time
	RevisionString = "?"
	// RevisionURLString is the full URL to the revision set at build time
	RevisionURLString = "?"
	// GeneratedString is the build date set at build time
	GeneratedString = "?"
	// CopyrightString is the copyright set at build time
	CopyrightString = "?"
)

func init() {
	cli.VersionPrinter = customVersionPrinter
	_ = os.Setenv("VERSION", VersionString)
	_ = os.Setenv("REVISION", RevisionString)
	_ = os.Setenv("GENERATED", GeneratedString)
}

func customVersionPrinter(c *cli.Context) {
	fmt.Printf("%v v=%v rev=%v d=%v\n", filepath.Base(c.App.Name),
		VersionString, RevisionString, GeneratedString)
}

func main() {
	app := &cli.App{
		Name:    "collectd-vsphere",
		Usage:   "forward metrics from vSphere events to collectd",
		Version: VersionString,
		Action:  mainAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "collectd-hostport",
				Usage:   "the host:port for collectd",
				EnvVars: []string{"COLLECTD_HOSTPORT"},
			},
			&cli.StringFlag{
				Name:    "collectd-username",
				Usage:   "the username for collectd",
				EnvVars: []string{"COLLECTD_USERNAME"},
			},
			&cli.StringFlag{
				Name:    "collectd-password",
				Usage:   "the password for collectd",
				EnvVars: []string{"COLLECTD_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "vsphere-url",
				Usage:   "the URL for the vSphere API",
				EnvVars: []string{"VSPHERE_URL"},
			},
			&cli.BoolFlag{
				Name:    "vsphere-insecure",
				Usage:   "connect to vSphere without verifying TLS certs",
				EnvVars: []string{"VSPHERE_INSECURE"},
			},
			&cli.StringFlag{
				Name:    "vsphere-cluster",
				Usage:   "path to the vSphere cluster to monitor events on",
				EnvVars: []string{"VSPHERE_CLUSTER"},
			},
		},
	}

	app.Run(os.Args)
}

func mainAction(c *cli.Context) error {
	statWriter, err := network.Dial(c.String("collectd-hostport"), network.ClientOptions{
		SecurityLevel: network.Encrypt,
		Username:      c.String("collectd-username"),
		Password:      c.String("collectd-password"),
	})
	if err != nil {
		log.Fatalf("couldn't connect to collectd: %v", err)
	}

	statsCollector := collectdvsphere.NewStatsCollector(statWriter, time.Minute)

	u, err := url.Parse(c.String("vsphere-url"))
	if err != nil {
		log.Fatalf("couldn't parse VSPHERE_URL: %v", err)
	}
	eventListener := collectdvsphere.NewVSphereEventListener(collectdvsphere.VSphereConfig{
		URL:         u,
		Insecure:    c.Bool("vsphere-insecure"),
		ClusterPath: c.String("vsphere-cluster"),
	}, statsCollector)

	eventListener.Start()

	return nil
}
