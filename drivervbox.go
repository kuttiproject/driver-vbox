package drivervbox

import (
	"fmt"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

func init() {
	driver, err := newvboxdriver()
	if err == nil {
		drivercore.RegisterDriver("vbox", driver)
	}
}

func newvboxdriver() (*Driver, error) {
	result := &Driver{}

	// find VBoxManage tool and set it
	vbmpath, err := findvboxmanage()
	if err != nil {
		result.status = "Error"
		result.errormessage = err.Error()
		return result, err
	}
	result.vboxmanagepath = vbmpath

	// test VBoxManage version
	vbmversion, err := workspace.Runwithresults(vbmpath, "--version")
	if err != nil {
		result.status = "Error"
		result.errormessage = err.Error()
		return result, err
	}
	var majorversion int
	_, err = fmt.Sscanf(vbmversion, "%d", &majorversion)
	if err != nil || majorversion < 6 {
		err = fmt.Errorf("unsupported VBoxManage version %v. 6.0 and above are supported", vbmversion)
		result.status = "Error"
		result.errormessage = err.Error()
		return result, err
	}

	result.status = "Ready"
	return result, nil
}
