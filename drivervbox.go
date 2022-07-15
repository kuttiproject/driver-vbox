package drivervbox

import (
	"github.com/kuttiproject/drivercore"
)

func init() {
	driver := &Driver{} //newvboxdriver()

	drivercore.RegisterDriver(driverName, driver)
}

// func newvboxdriver() *Driver {
// 	result := &Driver{}

// 	// find VBoxManage tool and set it
// 	vbmpath, err := findvboxmanage()
// 	if err != nil {
// 		result.status = "Error"
// 		result.errormessage = err.Error()
// 		return result
// 	}

// 	result.vboxmanagepath = vbmpath

// 	// test VBoxManage version
// 	vbmversion, err := workspace.Runwithresults(vbmpath, "--version")
// 	if err != nil {
// 		result.status = "Error"
// 		result.errormessage = err.Error()
// 		return result
// 	}
// 	var majorversion int
// 	_, err = fmt.Sscanf(vbmversion, "%d", &majorversion)
// 	if err != nil || majorversion < 6 {
// 		err = fmt.Errorf("unsupported VBoxManage version %v. 6.0 and above are supported", vbmversion)
// 		result.status = "Error"
// 		result.errormessage = err.Error()
// 		return result
// 	}

// 	result.status = "Ready"
// 	return result
// }
