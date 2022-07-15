package drivervbox

import (
	"fmt"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// Machine implements the drivercore.Machine interface for VirtualBox
type Machine struct {
	driver *Driver

	name string
	// netname        string
	clustername    string
	savedipaddress string
	status         drivercore.MachineStatus
	errormessage   string
}

// Name is the name of the machine.
func (vh *Machine) Name() string {
	return vh.name
}

func (vh *Machine) qname() string {
	return vh.driver.QualifiedMachineName(vh.name, vh.clustername)
}

func (vh *Machine) netname() string {
	return vh.driver.QualifiedNetworkName(vh.clustername)
}

// Status can be drivercore.MachineStatusRunning, drivercore.MachineStatusStopped
// drivercore.MachineStatusUnknown or drivercore.MachineStatusError.
func (vh *Machine) Status() drivercore.MachineStatus {
	return vh.status
}

// Error returns the last error caused when manipulating this machine.
// A valid value can be expected only when Status() returns
// drivercore.MachineStatusError.
func (vh *Machine) Error() string {
	return vh.errormessage
}

// IPAddress returns the current IP Address of this Machine.
// The Machine status has to be Running.
func (vh *Machine) IPAddress() string {
	// This guestproperty is only available if the VM is
	// running, and has the Virtual Machine additions enabled
	result, _ := vh.getproperty(propIPAddress)
	return trimpropend(result)
}

// SSHAddress returns the address and port number to SSH into this Machine.
func (vh *Machine) SSHAddress() string {
	// This guestproperty is set when the SSH port is forwarded
	result, _ := vh.getproperty(propSSHAddress)
	return trimpropend(result)
}

// Start starts a Machine.
// It does this by running the command:
//   VBoxManage startvm <machinename> --type headless
// Note that a Machine may not be ready for further operations at the end of this,
// and therefore its status will not change.
// See WaitForStateChange().
func (vh *Machine) Start() error {
	output, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"startvm",
		vh.qname(),
		"--type",
		"headless",
	)

	if err != nil {
		return fmt.Errorf("could not start the host '%s': %v. Output was %s", vh.name, err, output)
	}

	return nil
}

// Stop stops a Machine.
// It does this by running the command:
//   VBoxManage controlvm <machinename> acpipowerbutton
// Note that a Machine may not be ready for further operations at the end of this,
// and therefore its status will not change.
// See WaitForStateChange().
func (vh *Machine) Stop() error {
	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"controlvm",
		vh.qname(),
		"acpipowerbutton",
	)

	if err != nil {
		return fmt.Errorf("could not stop the host '%s': %v", vh.name, err)
	}

	// Big risk. Deleteing the LoggedInUser property that is used
	// to check running status. Should be ok, because starting a
	// VirtualBox VM is supposed to recreate that property.
	vh.unsetproperty(propLoggedInUsers)

	return nil
}

// ForceStop stops a Machine forcibly.
// It does this by running the command:
//   VBoxManage controlvm <machinename> poweroff
// This operation will set the status to drivercore.MachineStatusStopped.
func (vh *Machine) ForceStop() error {
	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"controlvm",
		vh.qname(),
		"poweroff",
	)

	if err != nil {
		return fmt.Errorf("could not force stop the host '%s': %v", vh.name, err)
	}

	// Big risk. Deleteing the LoggedInUser property that is used
	// to check running status. Should be ok, because starting a
	// VM host is supposed to recreate that property.
	vh.unsetproperty(propLoggedInUsers)

	vh.status = drivercore.MachineStatusStopped
	return nil
}

// WaitForStateChange waits the specified number of seconds,
// or until the Machine status changes.
// It does this by running the command:
//   VBoxManage guestproperty wait <machinename> /VirtualBox/GuestInfo/OS/LoggedInUsers --timeout <milliseconds> --fail-on-timeout
// WaitForStateChange should be called after a call to Start, before
// any other operation. From observation, it should not be called _before_ Stop.
func (vh *Machine) WaitForStateChange(timeoutinseconds int) {
	workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"guestproperty",
		"wait",
		vh.qname(),
		propLoggedInUsers,
		"--timeout",
		fmt.Sprintf("%v", timeoutinseconds*1000),
		"--fail-on-timeout",
	)

	vh.get()
}

func (vh *Machine) forwardingrulename(machineport int) string {
	return fmt.Sprintf("Node %s Port %d", vh.qname(), machineport)
}

// ForwardPort creates a rule to forward the specified Machine port to the
// specified physical host port. It does this by running the command:
//   VBoxManage natnetwork modify --netname <networkname> --port-forward-4 <rule>
// Port forwarding rule format is:
//   <rule name>:<protocol>:[<host ip>]:<host port>:[<guest ip>]:<guest port>
// The brackets [] are to be taken literally.
// This driver writes the rule name as "Node <machinename> Port <machineport>".
//
// So a sample rule would look like this:
//
//   Node node1 Port 80:TCP:[]:18080:[192.168.125.11]:80
func (vh *Machine) ForwardPort(hostport int, machineport int) error {
	forwardingrule := fmt.Sprintf(
		"%s:tcp:[]:%d:[%s]:%d",
		vh.forwardingrulename(machineport),
		hostport,
		vh.savedipAddress(),
		machineport,
	)

	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"natnetwork",
		"modify",
		"--netname",
		vh.netname(),
		"--port-forward-4",
		forwardingrule,
	)

	if err != nil {
		return fmt.Errorf(
			"could not create port forwarding rule %s for node %s on network %s: %v",
			forwardingrule,
			vh.name,
			vh.netname(),
			err,
		)
	}

	return nil
}

// UnforwardPort removes the rule which forwarded the specified VM host port.
// It does this by running the command:
//   VBoxManage natnetwork modify --netname <networkname> --port-forward-4 delete <rulename>
// This driver writes the rule name as "Node <machinename> Port <machineport>".
func (vh *Machine) UnforwardPort(machineport int) error {
	rulename := vh.forwardingrulename(machineport)
	_, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"natnetwork",
		"modify",
		"--netname",
		vh.netname(),
		"--port-forward-4",
		"delete",
		rulename,
	)

	if err != nil {
		return fmt.Errorf(
			"driver returned error while removing port forwarding rule %s for VM %s on network %s: %v",
			rulename,
			vh.name,
			vh.netname(),
			err,
		)
	}

	return nil
}

// ForwardSSHPort forwards the SSH port of this Machine to the specified
// physical host port. See ForwardPort() for details.
func (vh *Machine) ForwardSSHPort(hostport int) error {
	err := vh.ForwardPort(hostport, 22)
	if err != nil {
		return fmt.Errorf(
			"could not create SSH port forwarding rule for node %s on network %s: %v",
			vh.name,
			vh.netname(),
			err,
		)
	}

	sshaddress := fmt.Sprintf("localhost:%d", hostport)
	err = vh.setproperty(
		propSSHAddress,
		sshaddress,
	)
	if err != nil {
		return fmt.Errorf(
			"could not save SSH address for node %s : %v",
			vh.name,
			err,
		)
	}

	return nil
}

// ImplementsCommand returns true if the driver implements the specified predefined command.
// The vbox driver implements drivercore.RenameMachine
func (vh *Machine) ImplementsCommand(command drivercore.PredefinedCommand) bool {
	_, ok := vboxCommands[command]
	return ok

}

// ExecuteCommand executes the specified predefined command.
func (vh *Machine) ExecuteCommand(command drivercore.PredefinedCommand, params ...string) error {
	commandfunc, ok := vboxCommands[command]
	if !ok {
		return fmt.Errorf(
			"command '%v' not implemented",
			command,
		)
	}

	return commandfunc(vh, params...)
}

func (vh *Machine) get() error {
	output, err := workspace.Runwithresults(
		vh.driver.vboxmanagepath,
		"guestproperty",
		"enumerate",
		vh.qname(),
		"--patterns",
		"/VirtualBox/GuestInfo/Net/0/*|/kutti/*|/VirtualBox/GuestInfo/OS/LoggedInUsers",
	)

	if err != nil {
		return fmt.Errorf("machine %s not found", vh.name)
	}

	// If machine properties could be retrieved, assume the machine is in
	// Stopped state. Parsing the properties may change this.
	vh.status = drivercore.MachineStatusStopped

	if output != "" {
		vh.parseProps(output)
	}

	return nil
}

func (vh *Machine) savedipAddress() string {
	// This guestproperty is set when the VM is created
	if vh.savedipaddress != "" {
		return vh.savedipaddress
	}

	result, _ := vh.getproperty(propSavedIPAddress)
	return trimpropend(result)
}
