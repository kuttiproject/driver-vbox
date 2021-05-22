package drivervbox

const (
	driverName         = "vbox"
	driverDescription  = "Kutti driver for VirtualBox >=6.0"
	networkNameSuffix  = "kuttinet"
	networkNamePattern = "*" + networkNameSuffix
	dhcpaddress        = "192.168.125.3"
	dhcpnetmask        = "255.255.255.0"
	ipNetAddr          = "192.168.125"
	iphostbase         = 10
	forwardedPortBase  = 10000
)

// DefaultNetCIDR is the address range used by NAT networks.
var DefaultNetCIDR = "192.168.125.0/24"

// Driver implements the drivercore.Driver interface for VirtualBox.
type Driver struct {
	vboxmanagepath string
	status         string
	errormessage   string
}

// Name returns the "vbox"
func (vd *Driver) Name() string {
	return driverName
}

// Description returns "Kutti driver for VirtualBox >=6.0"
func (vd *Driver) Description() string {
	return driverDescription
}

// UsesNATNetworking returns true
func (vd *Driver) UsesNATNetworking() bool {
	return true
}

// Status returns current driver status
func (vd *Driver) Status() string {
	return vd.status
}

func (vd *Driver) Error() string {
	panic("not implemented") // TODO: Implement
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}
