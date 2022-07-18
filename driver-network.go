package drivervbox

import (
	"fmt"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// QualifiedNetworkName adds a 'kuttinet' suffix to the specified cluster name.
func (vd *Driver) QualifiedNetworkName(clustername string) string {
	return clustername + "kuttinet"
}

// DeleteNetwork deletes a network.
// It does this by running the command:
//   VBoxManage natnetwork remove --netname <networkname>
func (vd *Driver) DeleteNetwork(clustername string) error {
	if !vd.validate() {
		return vd
	}

	netname := vd.QualifiedNetworkName(clustername)

	output, err := workspace.RunWithResults(
		vd.vboxmanagepath,
		"natnetwork",
		"remove",
		"--netname",
		netname,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete NAT network %s:%v:%s",
			netname,
			err,
			output,
		)
	}

	// Associated dhcpserver must also be deleted
	output, err = workspace.RunWithResults(
		vd.vboxmanagepath,
		"dhcpserver",
		"remove",
		"--netname",
		netname,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete DHCP server %s:%v:%s",
			netname,
			err,
			output,
		)
	}

	return nil
}

// NewNetwork creates a new VirtualBox NAT network.
// It uses the CIDR common to all Kutti networks, and is dhcp-enabled at start.
func (vd *Driver) NewNetwork(clustername string) (drivercore.Network, error) {
	if !vd.validate() {
		return nil, vd
	}

	netname := vd.QualifiedNetworkName(clustername)

	// Multiple VirtualBox NAT Networks can have the same IP range
	// So, all Kutti networks will use the same network CIDR
	// We start with dhcp enabled.
	output, err := workspace.RunWithResults(
		vd.vboxmanagepath,
		"natnetwork",
		"add",
		"--netname",
		netname,
		"--network",
		DefaultNetCIDR,
		"--enable",
		"--dhcp",
		"on",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create NAT network %s:%v:%s",
			netname,
			err,
			output,
		)
	}

	// Manually create the associated DHCP server
	// Hard-coding a thirty-node limit for now
	output, err = workspace.RunWithResults(
		vd.vboxmanagepath,
		"dhcpserver",
		"add",
		"--netname",
		netname,
		"--ip",
		dhcpaddress,
		"--netmask",
		dhcpnetmask,
		"--lowerip",
		fmt.Sprintf("%s.%d", ipNetAddr, iphostbase),
		"--upperip",
		fmt.Sprintf("%s.%d", ipNetAddr, iphostbase+29),
		"--enable",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create DHCP server for network %s:%v:%s",
			netname,
			err,
			output,
		)
	}

	newnetwork := &Network{
		name:    netname,
		netCIDR: DefaultNetCIDR,
	}

	return newnetwork, err
}
