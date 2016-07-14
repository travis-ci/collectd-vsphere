# collectd-vsphere

Collects metrics from vSphere and sends them to collectd.

## Metrics

- `host/vsphere/operations-power_on_success`: number of successful VM power on events
- `host/vsphere/operations-power_on_failure`: number of failed VM power on events
- `host/vsphere/operations-power_off_success`: number of successful VM power off events
- `host/vsphere/operations-power_off_failure`: number of failed VM power off events
- `base-vm/vsphere/operations-clone_success`: number of successful VM clone events
- `base-vm/vsphere/operations-clone_failure`: number of failed VM clone events

## Config

Make sure to set up the network plugin in collectd.

```
export VSPHERE_URL="https://user:password@vsphere/sdk"
export VSPHERE_INSECURE="false" # or "true" if you need
export VSPHERE_CLUSTER="/MyDC/host/MyCluster/"
export COLLECTD_HOSTPORT="127.0.0.1:12345"
export COLLECTD_USERNAME="some-username"
export COLLECTD_PASSWORD="some-password"
```

## License

See LICENSE file.