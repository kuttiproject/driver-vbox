package drivervbox

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// QualifiedNetworkName adds a 'kuttinet' suffix to the specified cluster name.
func (vd *Driver) QualifiedNetworkName(clustername string) string {
	return clustername + "kuttinet"
}

/*ListNetworks parses the list of NAT networks returned by
    VBoxManage natnetwork list
As of VBoxManage 6.0.8r130520, the format is:

  NAT Networks:

  Name:        KubeNet
  Network:     10.0.2.0/24
  Gateway:     10.0.2.1
  IPv6:        No
  Enabled:     Yes


  Name:        NatNetwork
  Network:     10.0.2.0/24
  Gateway:     10.0.2.1
  IPv6:        No
  Enabled:     Yes


  Name:        NatNetwork1
  Network:     10.0.2.0/24
  Gateway:     10.0.2.1
  IPv6:        No
  Enabled:     Yes

  3 networks found

Note the blank lines: one before and after
each network. If there are zero networks, the output is:

  NAT Networks:

  0 networks found


*/
func (vd *Driver) ListNetworks() ([]drivercore.Network, error) {
	if !vd.validate() {
		return nil, vd
	}

	// The default pattern for all our network names is "*kuttinet"
	output, err := workspace.Runwithresults(
		vd.vboxmanagepath,
		"natnetwork",
		"list",
		"*kuttinet",
	)
	if err != nil {
		return nil, err
	}

	// TODO: write a better parser
	lines := strings.Split(output, "\n")
	numlines := len(lines)
	if numlines < 4 {
		// Bare mininum output should be
		//   NAT Networks:
		//
		//   0 networks found
		//
		return nil, errors.New("could not recognise VBoxManage output for natnetworks list while getting lines")
	}

	var numnetworks int

	_, err = fmt.Sscanf(lines[numlines-2], "%d", &numnetworks)
	if err != nil {
		return nil, errors.New("could not recognise VBoxManage output for natnetworks list while getting count")
	}

	justlines := lines[2 : numlines-2]
	numlines = len(justlines)

	result := make([]drivercore.Network, numnetworks)

	for i, j := 0, 0; i < numlines; i, j = i+7, j+1 {
		result[j] = &Network{
			name:    justlines[i][13:],
			netCIDR: justlines[i+1][13:],
		}
	}

	return result, nil
}

// GetNetwork returns a network, or an error.
func (vd *Driver) GetNetwork(clustername string) (drivercore.Network, error) {
	netname := vd.QualifiedNetworkName(clustername)

	networks, err := vd.ListNetworks()
	if err != nil {
		return nil, err
	}

	for _, network := range networks {
		if network.Name() == netname {
			return network, nil
		}
	}

	return nil, fmt.Errorf("network %s not found", netname)
}

// DeleteNetwork deletes a network.
// It does this by running the command:
//   VBoxManage natnetwork remove --netname <networkname>
func (vd *Driver) DeleteNetwork(clustername string) error {
	if !vd.validate() {
		return vd
	}

	netname := vd.QualifiedNetworkName(clustername)

	output, err := workspace.Runwithresults(
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
	output, err = workspace.Runwithresults(
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
	output, err := workspace.Runwithresults(
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
	output, err = workspace.Runwithresults(
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
