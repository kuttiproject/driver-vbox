package drivervbox

import (
	"github.com/kuttiproject/drivercore"
)

func init() {
	driver := &Driver{}

	drivercore.RegisterDriver(driverName, driver)
}
