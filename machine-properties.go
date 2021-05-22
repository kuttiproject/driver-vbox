package drivervbox

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

const (
	propIPAddress      = "/VirtualBox/GuestInfo/Net/0/V4/IP"
	propIPAddress2     = "/VirtualBox/GuestInfo/Net/1/V4/IP"
	propIPAddress3     = "/VirtualBox/GuestInfo/Net/2/V4/IP"
	propLoggedInUsers  = "/VirtualBox/GuestInfo/OS/LoggedInUsers"
	propSSHAddress     = "/kutti/VMInfo/SSHAddress"
	propSavedIPAddress = "/kutti/VMInfo/SavedIPAddress"
)

var (
	properrorpattern, _ = regexp.Compile("error: (.*)\n")
	proppattern, _      = regexp.Compile("Name: (.*), value: (.*), timestamp: (.*), flags:(.*)\n")
)

// When properties are parsed by parseprop(), certain properties
// can cause an action to be taken. This map contains the names of some
// VirtualBox properties, and correspoding actions.
var propMap = map[string]func(*Machine, string){
	propLoggedInUsers: func(vh *Machine, value string) {
		vh.status = drivercore.MachineStatusRunning
	},
	propSavedIPAddress: func(vh *Machine, value string) {
		vh.savedipaddress = trimpropend(value)
	},
}

func (vh *Machine) getproperty(propname string) (string, bool) {
	output, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"guestproperty",
		"get",
		vh.qname(),
		propname,
	)

	// VBoxManage guestproperty gets the hardcoded value "No value set!"
	// if the property value cannot be retrieved
	if err != nil || output == "No value set!" || output == "No value set!\n" {
		return "", false
	}

	// Output is in the format
	// Value: <value>
	// So, 7th rune onwards
	return output[7:], true
}

func (vh *Machine) setproperty(propname string, value string) error {
	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"guestproperty",
		"set",
		vh.qname(),
		propname,
		value,
	)

	if err != nil {
		// TODO: Error consolidation
		return fmt.Errorf(
			"could not set property %s for host %s: %v",
			propname,
			vh.name,
			err,
		)
	}

	return nil
}

func (vh *Machine) unsetproperty(propname string) error {
	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"guestproperty",
		"unset",
		vh.qname(),
		propname,
	)

	if err != nil {
		return fmt.Errorf(
			"could not unset property %s for host %s: %v",
			propname,
			vh.name,
			err,
		)
	}

	return nil
}

func trimpropend(s string) string {
	return strings.TrimSpace(s)
}

func (vh *Machine) parseProps(propstr string) {
	// There are two possibilities. Either:
	// VBoxManage: error: Could not find a registered machine named 'xxx'
	// ...
	// Or:
	// Name: /VirtualBox/GuestInfo/Net/0/V4/IP, value: 10.0.2.15, timestamp: 1568552111298588000, flags:
	// ...

	// This should not have made it this far. Still,
	// belt and suspenders...
	errorsfound := properrorpattern.FindAllStringSubmatch(propstr, 1)
	if len(errorsfound) != 0 {
		// deal with the error with:
		// errorsfound[0][1]
		vh.status = drivercore.MachineStatusError
		vh.errormessage = errorsfound[0][1]
		return
	}

	results := proppattern.FindAllStringSubmatch(propstr, -1)
	for _, record := range results {
		// record[1] - Name and record[2] - Value
		// Run any configured action for a property
		action, ok := propMap[record[1]]
		if ok {
			action(vh, record[2])
		}
	}
}
