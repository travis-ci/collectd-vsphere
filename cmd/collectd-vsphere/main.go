package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"collectd.org/network"
	"github.com/Sirupsen/logrus"
	raven "github.com/getsentry/raven-go"
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
				EnvVars: []string{"COLLECTD_VSPHERE_COLLECTD_HOSTPORT", "COLLECTD_HOSTPORT"},
			},
			&cli.StringFlag{
				Name:    "collectd-username",
				Usage:   "the username for collectd",
				EnvVars: []string{"COLLECTD_VSPHERE_COLLECTD_USERNAME", "COLLECTD_USERNAME"},
			},
			&cli.StringFlag{
				Name:    "collectd-password",
				Usage:   "the password for collectd",
				EnvVars: []string{"COLLECTD_VSPHERE_COLLECTD_PASSWORD", "COLLECTD_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "vsphere-url",
				Usage:   "the URL for the vSphere API",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_URL", "VSPHERE_URL"},
			},
			&cli.BoolFlag{
				Name:    "vsphere-insecure",
				Usage:   "connect to vSphere without verifying TLS certs",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_INSECURE", "VSPHERE_INSECURE"},
			},
			&cli.StringFlag{
				Name:    "vsphere-cluster",
				Usage:   "path to the vSphere cluster to monitor events on",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_CLUSTER", "VSPHERE_CLUSTER"},
			},
			&cli.StringSliceFlag{
				Name:    "vsphere-clusters",
				Usage:   "comma-separated paths to the vSphere clusters to monitor events on",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_CLUSTERS", "VSPHERE_CLUSTERS"},
			},
			&cli.StringFlag{
				Name:    "vsphere-base-vm-folder",
				Usage:   "path to the vSphere folder containing base VMs",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_BASE_VM_FOLDER", "VSPHERE_BASE_VM_FOLDER"},
			},
			&cli.StringSliceFlag{
				Name:    "vsphere-base-vm-folders",
				Usage:   "comma-separated paths to the vSphere folders containing base VMs",
				EnvVars: []string{"COLLECTD_VSPHERE_VSPHERE_BASE_VM_FOLDERS", "VSPHERE_BASE_VM_FOLDERS"},
			},
			&cli.StringFlag{
				Name:    "sentry-dsn",
				Usage:   "DSN for Sentry integration",
				EnvVars: []string{"COLLECTD_VSPHERE_SENTRY_DSN", "SENTRY_DSN"},
			},
			&cli.StringFlag{
				Name:    "collectd-plugin-instance",
				Usage:   "Plugin instance value for collectd metrics to be able to distinguish metrics from this instance of collectd-vsphere from other instances",
				EnvVars: []string{"COLLECTD_VSPHERE_COLLECTD_PLUGIN_INSTANCE", "COLLECTD_PLUGIN_INSTANCE"},
			},
		},
	}

	app.Run(os.Args)
}

func mainAction(c *cli.Context) error {
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	logger := logrus.WithField("pid", os.Getpid())
	logger.Info("collectd-vsphere starting")
	defer logger.Info("collectd-vsphere stopping")

	if c.IsSet("sentry-dsn") {
		err := raven.SetDSN(c.String("sentry-dsn"))
		if err != nil {
			logger.WithField("err", err).Error("couldn't set raven dsn")
		}
		raven.SetRelease(VersionString)
	}

	logger.Info("connecting to collectd")
	statWriter, err := network.Dial(c.String("collectd-hostport"), network.ClientOptions{
		SecurityLevel: network.Encrypt,
		Username:      c.String("collectd-username"),
		Password:      c.String("collectd-password"),
	})
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		logger.WithField("err", err).Fatal("couldn't connect to collectd")
	}

	statsCollector := collectdvsphere.NewStatsCollector(statWriter, time.Minute, logger, c.String("collectd-plugin-instance"))

	var clusterPaths []string
	if c.IsSet("vsphere-cluster") && c.IsSet("vsphere-clusters") {
		logger.Fatal("only one of vsphere-cluster and vsphere-clusters should be set")
	} else if c.IsSet("vsphere-cluster") {
		clusterPaths = []string{c.String("vsphere-cluster")}
	} else if c.IsSet("vsphere-clusters") {
		clusterPaths = c.StringSlice("vsphere-clusters")
	}

	var baseVMPaths []string
	if c.IsSet("vsphere-base-vm-folder") && c.IsSet("vsphere-base-vm-folders") {
		logger.Fatal("only one of vsphere-base-vm-folder and vsphere-base-vm-folders should be set")
	} else if c.IsSet("vsphere-base-vm-folder") {
		baseVMPaths = []string{c.String("vsphere-base-vm-folder")}
	} else if c.IsSet("vsphere-base-vm-folders") {
		baseVMPaths = c.StringSlice("vsphere-base-vm-folders")
	}

	u, err := url.Parse(c.String("vsphere-url"))
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		logger.WithField("err", err).Fatal("couldn't parse vsphere url")
	}
	eventListener := collectdvsphere.NewVSphereEventListener(collectdvsphere.VSphereConfig{
		URL:          u,
		Insecure:     c.Bool("vsphere-insecure"),
		ClusterPaths: clusterPaths,
		BaseVMPaths:  baseVMPaths,
	}, statsCollector, logger.WithField("component", "vsphere-event-listener"))

	panicErr, _ := raven.CapturePanicAndWait(func() {
		err := eventListener.Start()
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			logger.WithField("err", err).Fatal("event listener errored")
		}
	}, nil)
	if panicErr != nil {
		logger.WithField("err", panicErr).Fatal("eventListener paniced, exiting")
	}

	return nil
}
