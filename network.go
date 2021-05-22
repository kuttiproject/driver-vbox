package drivervbox

// Network implements the VMNetwork interface for VirtualBox.
type Network struct {
	name    string
	netCIDR string
}

// Name is the name of the network.
func (vn *Network) Name() string {
	return vn.name
}

// CIDR is the network's IPv4 address range.
func (vn *Network) CIDR() string {
	return vn.netCIDR
}

// SetCIDR is not implemented for the VirtualBox driver.
func (vn *Network) SetCIDR(cidr string) {
	panic("not implemented")
}
