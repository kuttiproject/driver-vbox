module github.com/kuttiproject/driver-vbox

go 1.16

require (
	github.com/kuttiproject/drivercore v0.3.0
	github.com/kuttiproject/kuttilog v0.2.0
	github.com/kuttiproject/workspace v0.3.0
)

retract (
	v0.2.0 // Bug in driver registration
	[v0.1.0, v0.1.2] // Image source changed to kuttiproject/driver-vbox-images@0.2
)
