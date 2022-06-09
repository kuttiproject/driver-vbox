package drivervbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/kuttilog"
	"github.com/kuttiproject/workspace"
)

func machinesBaseDir() (string, error) {
	return workspace.Cachesubdir("driver-vbox-machines")
}

// QualifiedMachineName returns a name in the form <clustername>-<machinename>
func (vd *Driver) QualifiedMachineName(machinename string, clustername string) string {
	return clustername + "-" + machinename
}

/*ListMachines parses the list of VMs returned by
    VBoxManage list vms
As of VBoxManage 6.0.8r130520, the format is:

  "Matsya" {e3509073-d188-4cca-8eaf-cb9f3be7ac4a}
  "Krishna" {5d9b1b16-5059-42ae-a160-e93b470f940e}
  "one" {06748689-7f4e-4915-8fbf-6111596f85a2}
  "two" {eee169a7-09eb-473e-96be-5d37868c5d5e}
  "minikube" {5bf78b43-3240-4f50-911b-fbc111d4d085}
  "Node 1" {53b82a61-ae52-44c2-86d5-4c686502dd64}

*/
func (vd *Driver) ListMachines() ([]drivercore.Machine, error) {
	output, err := workspace.Runwithresults(
		vd.vboxmanagepath,
		"list",
		"vms",
	)
	if err != nil {
		return nil, fmt.Errorf("could not get list of VMs: %v", err)
	}

	// TODO: Write a better parser
	result := []drivercore.Machine{}
	lines := strings.Split(output, "\n")
	if len(lines) < 1 {
		return result, nil
	}

	actualcount := 0
	for _, value := range lines {
		line := strings.Split(value, " ")
		if len(line) == 2 {
			result = append(result, &Machine{
				driver: vd,
				name:   trimQuotes(line[0]),
				status: drivercore.MachineStatusUnknown,
			})
			actualcount++
		}
	}

	return result[:actualcount], err
}

// GetMachine returns the named machine, or an error.
// It does this by running the command:
//   VBoxManage guestproperty enumerate <machinename> --patterns "/VirtualBox/GuestInfo/Net/0/*|/kutti/*|/VirtualBox/GuestInfo/OS/LoggedInUsers"
// and parsing the enumerated properties.
func (vd *Driver) GetMachine(machinename string, clustername string) (drivercore.Machine, error) {
	machine := &Machine{
		driver:      vd,
		name:        machinename,
		clustername: clustername,
		status:      drivercore.MachineStatusUnknown,
	}

	err := machine.get()

	if err != nil {
		return nil, err
	}

	return machine, nil
}

// DeleteMachine completely deletes a Machine.
// It does this by running the command:
//   VBoxManage unregistervm "<hostname>" --delete
func (vd *Driver) DeleteMachine(machinename string, clustername string) error {
	qualifiedmachinename := vd.QualifiedMachineName(machinename, clustername)
	output, err := workspace.Runwithresults(
		vd.vboxmanagepath,
		"unregistervm",
		qualifiedmachinename,
		"--delete",
	)

	if err != nil {
		return fmt.Errorf("could not delete machine %s: %v:%s", machinename, err, output)
	}

	return nil
}

var ipRegex, _ = regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

// NewMachine creates a VM, and connects it to a previously created NAT network.
// It also starts the VM, changes the hostname, saves the IP address, and stops
// it again.
// It runs the following two VBoxManage commands, in order:
//   VBoxManage import <nodeimageovafile> --vsys 0 --vmname "<hostname>"
//   VBoxManage modifyvm "<hostname>" --nic1 natnetwork --nat-network1 <networkname>
// The first imports from an .ova file (easiest way to get fully configured VM), while
// setting the VM name. The second connects the first network interface card to
// the NAT network.
// This function may return nil and an error, or a Machine and an error.
// In the second case, if the caller does not actually want the machine, they should
// call DeleteMachine afterwards.
func (vd *Driver) NewMachine(machinename string, clustername string, k8sversion string) (drivercore.Machine, error) {
	qualifiedmachinename := vd.QualifiedMachineName(machinename, clustername)

	kuttilog.Println(2, "Importing image...")

	ovafile, err := imagepathfromk8sversion(k8sversion)
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(ovafile); err != nil {
		return nil, fmt.Errorf("could not retrieve image %s: %v", ovafile, err)
	}

	machinebasedir, err := machinesBaseDir()
	if err != nil {
		return nil, err
	}

	// Risk: convert path to absolute
	absmachinebasedir, err := filepath.Abs(machinebasedir)
	if err != nil {
		return nil, err
	}

	l, err := workspace.Runwithresults(
		vd.vboxmanagepath,
		"import",
		ovafile,
		"--vsys",
		"0",
		"--vmname",
		qualifiedmachinename,
		"--vsys",
		"0",
		"--group",
		"/"+clustername,
		"--vsys",
		"0",
		"--basefolder",
		absmachinebasedir,
	)

	if err != nil {
		return nil, fmt.Errorf("could not import ovafile %s: %v(%v)", ovafile, err, l)
	}

	// Attach newly created VM to NAT Network
	kuttilog.Println(2, "Attaching host to network...")
	newmachine := &Machine{
		driver:      vd,
		name:        machinename,
		clustername: clustername,
		status:      drivercore.MachineStatusStopped,
	}
	networkname := vd.QualifiedNetworkName(clustername)

	_, err = workspace.Runwithresults(
		vd.vboxmanagepath,
		"modifyvm",
		newmachine.qname(),
		"--nic1",
		"natnetwork",
		"--nat-network1",
		networkname,
	)

	if err != nil {
		newmachine.status = drivercore.MachineStatusError
		newmachine.errormessage = fmt.Sprintf("Could not attach node %s to network %s: %v", machinename, networkname, err)
		return newmachine, fmt.Errorf("could not attach node %s to network %s: %v", machinename, networkname, err)
	}

	// Start the host
	kuttilog.Println(2, "Starting host...")
	err = newmachine.Start()
	if err != nil {
		return newmachine, err
	}
	// TODO: Try to parameterize the timeout
	newmachine.WaitForStateChange(25)

	// Change the name
	for renameretries := 1; renameretries < 4; renameretries++ {
		kuttilog.Printf(2, "Renaming host (attempt %v/3)...", renameretries)
		err = renamemachine(newmachine, machinename)
		if err == nil {
			break
		}
		kuttilog.Printf(2, "Failed. Waiting %v seconds before retry...", renameretries*10)
		time.Sleep(time.Duration(renameretries*10) * time.Second)
	}

	if err != nil {
		return newmachine, err
	}
	kuttilog.Println(2, "Host renamed.")

	// Save the IP Address
	// The first IP address should be DHCP-assigned, and therefore start with
	// ipNetAddr (192.168.125 by default). This may fail if we check too soon.
	// In some cases, VirtualBox picks up other interfaces first. So, we check
	// up to three interfaces for the correct IP address, and do this up to 3
	// times.
	ipSet := false
	for ipretries := 1; ipretries < 4; ipretries++ {
		kuttilog.Printf(2, "Fetching IP address (attempt %v/3)...", ipretries)

		var ipaddress string
		ipprops := []string{propIPAddress, propIPAddress2, propIPAddress3}

		for _, ipprop := range ipprops {
			ipaddr, present := newmachine.getproperty(ipprop)

			if present {
				ipaddr = trimpropend(ipaddr)
				if ipRegex.MatchString(ipaddr) && strings.HasPrefix(ipaddr, ipNetAddr) {
					ipaddress = ipaddr
					break
				}
			}

			if kuttilog.V(4) {
				kuttilog.Printf(4, "value of property %v is %v, and present is %v.", ipprop, ipaddr, present)
				kuttilog.Printf(4, "Regex match is %v, and prefix match is %v.", ipRegex.MatchString(ipaddr), strings.HasPrefix(ipaddr, ipNetAddr))
			}
		}

		if ipaddress != "" {
			kuttilog.Printf(2, "Obtained IP address '%v'", ipaddress)
			newmachine.setproperty(propSavedIPAddress, ipaddress)
			ipSet = true
			break
		}

		kuttilog.Printf(2, "Failed. Waiting %v seconds before retry...", ipretries*10)
		time.Sleep(time.Duration(ipretries*10) * time.Second)
	}

	if !ipSet {
		kuttilog.Printf(0, "Error: Failed to get IP address. You may have to delete this node and recreate it manually.")
	}

	kuttilog.Println(2, "Stopping host...")
	newmachine.Stop()
	// newhost.WaitForStateChange(25)

	newmachine.status = drivercore.MachineStatusStopped

	return newmachine, nil
}
