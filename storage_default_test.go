//go:build !darwin || !arm64

package drivervbox

import "testing"

// These are the values for the platforms that use driver-vbox-images. Apple
// silicon has its own, in storage_mac_m_test.go. A change to the version of
// either repository is a change to its test.
func TestDefaultImagesVersionAndSourceURL(t *testing.T) {
	if ImagesVersion != "0.5" {
		t.Errorf("ImagesVersion is %q", ImagesVersion)
	}

	want := "https://github.com/kuttiproject/driver-vbox-images/releases/download/v0.5/driver-vbox-images.json"
	if ImagesSourceURL != want {
		t.Errorf("ImagesSourceURL is %q, wanted %q", ImagesSourceURL, want)
	}
}
