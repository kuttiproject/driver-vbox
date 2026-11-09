//go:build darwin && arm64

package drivervbox

import "testing"

// These are the values for Apple silicon, which uses driver-vbox-arm64-images,
// at a version of its own. The other platforms have theirs, in
// storage_default_test.go. A change to the version of either repository is a
// change to its test.
func TestAppleSiliconImagesVersionAndSourceURL(t *testing.T) {
	if ImagesVersion != "0.4" {
		t.Errorf("ImagesVersion is %q", ImagesVersion)
	}

	want := "https://github.com/kuttiproject/driver-vbox-arm64-images/releases/download/v0.4/driver-vbox-images.json"
	if ImagesSourceURL != want {
		t.Errorf("ImagesSourceURL is %q, wanted %q", ImagesSourceURL, want)
	}
}
