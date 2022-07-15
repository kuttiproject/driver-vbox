//go:build !windows

package drivervbox

import (
	"errors"
	"os/exec"
)

func findvboxmanage() (string, error) {

	// Try looking it up on the path
	toolpath, err := exec.LookPath("VBoxManage")
	if err == nil {
		return toolpath, nil
	}

	// Give up
	return "", errors.New(
		"VBoxManage not found. Please ensure that Oracle VirtualBox 6.0 or greater is installed, and the VBoxManage utility is on your PATH",
	)
}
