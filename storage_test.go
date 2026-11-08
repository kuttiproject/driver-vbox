//go:build !darwin || !arm64

package drivervbox

import "testing"

func TestDefaultImagesVersionAndSourceURL(t *testing.T) {
	if ImagesVersion != "0.5" {
		t.Errorf("ImagesVersion is %q", ImagesVersion)
	}

	want := "https://github.com/kuttiproject/driver-vbox-images/releases/download/v0.5/driver-vbox-images.json"
	if ImagesSourceURL != want {
		t.Errorf("ImagesSourceURL is %q, wanted %q", ImagesSourceURL, want)
	}
}
