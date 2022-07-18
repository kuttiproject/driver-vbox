package drivervbox

import (
	"fmt"

	"github.com/kuttiproject/workspace"
)

const (
	driverName         = "vbox"
	driverDescription  = "Kutti driver for VirtualBox 6.0 and above"
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
	validated      bool
	status         string
	errormessage   string
}

// Name returns "vbox"
func (vd *Driver) Name() string {
	return driverName
}

// Description returns "Kutti driver for VirtualBox 6.0 and above"
func (vd *Driver) Description() string {
	return driverDescription
}

// UsesPerClusterNetworking returns true
func (vd *Driver) UsesPerClusterNetworking() bool {
	return true
}

// UsesNATNetworking returns true
func (vd *Driver) UsesNATNetworking() bool {
	return true
}

func (vd *Driver) validate() bool {
	if vd.validated {
		return true
	}

	// find VBoxManage tool and set it
	vbmpath, err := findvboxmanage()
	if err != nil {
		vd.status = "Error"
		vd.errormessage = err.Error()
		return false
	}
	vd.vboxmanagepath = vbmpath

	// test VBoxManage version
	vbmversion, err := workspace.RunWithResults(vbmpath, "--version")
	if err != nil {
		vd.status = "Error"
		vd.errormessage = err.Error()
		return false
	}
	var majorversion int
	_, err = fmt.Sscanf(vbmversion, "%d", &majorversion)
	if err != nil || majorversion < 6 {
		err = fmt.Errorf("unsupported VBoxManage version %v. 6.0 and above are supported", vbmversion)
		vd.status = "Error"
		vd.errormessage = err.Error()
		return false
	}

	vd.status = "Ready"
	vd.validated = true
	return true
}

// Status returns current driver status
func (vd *Driver) Status() string {
	vd.validate()
	return vd.status
}

func (vd *Driver) Error() string {
	vd.validate()
	if vd.status != "Error" {
		return ""
	}

	return vd.errormessage
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}
