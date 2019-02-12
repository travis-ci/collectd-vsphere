#!/bin/bash

main() {
  /opt/collectd/sbin/collectdmon -c /opt/collectd/sbin/collectd
  ./collectd-vsphere
}

write_collectd_config() {
  local conf=/opt/collectd/etc/collectd.conf
  cat <<EOF >$conf
# Set by Librato
Interval 60

LoadPlugin syslog
<Plugin syslog>
	LogLevel info
</Plugin>

LoadPlugin cpu
LoadPlugin interface
LoadPlugin load
LoadPlugin memory
LoadPlugin ping

EOF

  for host in $COLLECTD_VSPHERE_PING_AMQP_HOSTS; do
    cat <<EOF >>$conf
<Plugin ping>
	Host "$host"
	Interval 1.0
	Timeout 0.9
	MaxMissed 10
</Plugin>

EOF
  done

  for host in $COLLECTD_VSPHERE_PING_DNS_HOSTS; do
    cat <<EOF >>$conf
<Plugin ping>
	Host "$host"
	Interval 1.0
	Timeout 0.9
	MaxMissed -1
</Plugin>

EOF
  done

  cat <<EOF >>$conf
Include "/opt/collectd/etc/collectd.conf.d/librato.conf"

# REQUIRED for Cisco SNMP info
LoadPlugin snmp
Include "/opt/collectd/etc/collectd.conf.d/snmp.conf"

LoadPlugin network
<Plugin "network">
  <Listen "127.0.0.1" "1785">
    SecurityLevel "Encrypt"
    AuthFile "/opt/collectd/etc/collectd-network-auth"
  </Listen>
</Plugin>
EOF

  conf=/opt/collectd/etc/collectd.conf.d/librato.conf
  host=$(hostname)
  cat <<EOF >$conf
LoadPlugin write_http
<Plugin write_http>
  <Node "${host}">
    URL "https://collectd.librato.com/v1/measurements"
    Format "JSON"
    BufferSize 8192
    User "${COLLECTD_VSPHERE_LIBRATO_EMAIL}"
    Password "${COLLECTD_VSPHERE_LIBRATO_TOKEN}"
  </Node>
</Plugin>
EOF

  conf=/opt/collectd/etc/collectd.conf.d/snmp.conf
  cat <<EOF >$conf
<Plugin snmp>
	<Data "ifmib_if_octets64">
		Type "if_octets"
		Table true
		Instance "IF-MIB::ifName"
		Values "IF-MIB::ifHCInOctets" "IF-MIB::ifHCOutOctets"
	</Data>

	<Data "ifmib_if_packets64">
		Type "if_packets"
		Table true
		Instance "IF-MIB::ifName"
		Values "IF-MIB::ifHCInUcastPkts" "IF-MIB::ifHCOutUcastPkts"
	</Data>

  <Data "cpu_percentage">
    Type "percent"
    Table true
    Instance "HOST-RESOURCES-MIB::hrDeviceDescr"
    Values "HOST-RESOURCES-MIB::hrProcessorLoad"
  </Data>

	<Host "TravisCI-Prod-FW">
		Address "${COLLECTD_VSPHERE_FW_IP}"
		Version 2
		Community "${COLLECTD_VSPHERE_FW_SNMP_COMMUNITY}"
		Collect "ifmib_if_octets64" "ifmib_if_packets64"
		Interval 60
	</Host>
</Plugin>
EOF

  conf=/opt/collectd/etc/collectd-network-auth
  echo "$COLLECTD_VSPHERE_COLLECTD_USERNAME: $COLLECTD_VSPHERE_COLLECTD_PASSWORD" >$conf
}

write_collectd_config
main
