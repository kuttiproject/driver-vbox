module github.com/kuttiproject/driver-vbox

go 1.22

require (
	github.com/kuttiproject/drivercore v0.3.1
	github.com/kuttiproject/imagecore v0.1.1
	github.com/kuttiproject/kuttilog v0.2.1
	github.com/kuttiproject/workspace v0.4.0
)

retract (
	v0.5.1 // Bug in referenced imagecore
	v0.2.0 // Bug in driver registration
	[v0.1.0, v0.1.2] // Image source changed to kuttiproject/driver-vbox-images@0.2
)
