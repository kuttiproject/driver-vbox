package drivervbox

import (
	"fmt"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// TODO: Look at parameterizing these
var (
	vboxUsername = "kuttiadmin"
	vboxPassword = "Pass@word1"
)

// runwithresults allows running commands inside a VM Host.
// It does this by running the command:
// - VBoxManage guestcontrol <machiname> --username <username> --password <password> run -- <command line>
// This requires Virtual Machine Additions to be running in the guest operating system.
// The guest OS should be fully booted up.
func (vh *Machine) runwithresults(execpath string, paramarray ...string) (string, error) {
	params := []string{
		"guestcontrol",
		vh.qname(),
		"--username",
		vboxUsername,
		"--password",
		vboxPassword,
		"run",
		"--",
		execpath,
	}
	params = append(params, paramarray...)

	output, err := workspace.RunWithResults(
		vh.driver.vboxmanagepath,
		params...,
	)

	return output, err
}

var vboxCommands = map[drivercore.PredefinedCommand]func(*Machine, ...string) error{
	drivercore.RenameMachine: renamemachine,
}

func renamemachine(vh *Machine, params ...string) error {
	newname := params[0]
	execname := fmt.Sprintf("/home/%s/kutti-installscripts/set-hostname.sh", vboxUsername)

	_, err := vh.runwithresults(
		"/usr/bin/sudo",
		execname,
		newname,
	)

	return err
}
