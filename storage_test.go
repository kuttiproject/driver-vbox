package drivervbox

import (
	"strings"
	"testing"
)

// On every platform, the source URL is built from the version of the images
// repository, which is ImagesVersion, and from the name of the list file.
func TestImagesSourceURLIsBuiltFromImagesVersion(t *testing.T) {
	if !strings.HasPrefix(ImagesSourceURL, "https://github.com/kuttiproject/") {
		t.Errorf("ImagesSourceURL is %q", ImagesSourceURL)
	}

	suffix := "/releases/download/v" + ImagesVersion + "/driver-vbox-images.json"
	if !strings.HasSuffix(ImagesSourceURL, suffix) {
		t.Errorf("ImagesSourceURL is %q, expected it to end with %q", ImagesSourceURL, suffix)
	}
}
