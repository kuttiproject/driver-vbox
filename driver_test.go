package drivervbox

import (
	"strings"
	"testing"

	"github.com/kuttiproject/drivercore"
)

func TestDriverProperties(t *testing.T) {
	d := &Driver{}

	if d.Name() != "vbox" {
		t.Errorf("name is %q", d.Name())
	}
	if d.Description() != "Kutti driver for VirtualBox 7.1 and above" {
		t.Errorf("description is %q", d.Description())
	}
	if !d.UsesPerClusterNetworking() || !d.UsesNATNetworking() {
		t.Error("the driver uses both per-cluster and NAT networking")
	}
}

func TestDriverIsRegistered(t *testing.T) {
	d, ok := drivercore.GetDriver("vbox")
	if !ok {
		t.Fatal("the vbox driver is not registered")
	}
	if _, ok := d.(*Driver); !ok {
		t.Errorf("the registered driver is a %T", d)
	}
}

func TestStatusReadyWhenVBoxManageChecksOut(t *testing.T) {
	testStorage(t)
	d := &Driver{}

	if status := d.Status(); status != "Ready" {
		t.Errorf("status is %q, error is %q", status, d.Error())
	}
	if d.Error() != "" {
		t.Errorf("error is %q", d.Error())
	}

	// VBoxManage is asked for its version once, however often the driver is
	// asked for its status.
	d.Status()
	if calls := vbmCalls(t); len(calls) != 1 || calls[0] != "--version" {
		t.Errorf("the calls to VBoxManage were %q", calls)
	}
}

func TestStatusErrorWhenVBoxManageIsNotFound(t *testing.T) {
	testStorage(t)
	noVBoxManage(t)
	d := &Driver{}

	if status := d.Status(); status != "Error" {
		t.Errorf("status is %q", status)
	}
	if d.Error() != "VBoxManage not found" {
		t.Errorf("error is %q", d.Error())
	}
}

func TestStatusErrorWhenVBoxManageIsTooOld(t *testing.T) {
	testStorage(t)
	t.Setenv("FAKE_VBM_VERSION", "6.1.50r161033")
	d := &Driver{}

	if status := d.Status(); status != "Error" {
		t.Errorf("status is %q", status)
	}
	if !strings.Contains(d.Error(), "unsupported VBoxManage version") {
		t.Errorf("error is %q", d.Error())
	}
}
