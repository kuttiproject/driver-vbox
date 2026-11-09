//go:build darwin && arm64

package drivervbox

// ImagesVersion is the version of the driver-vbox-arm64-images repository
// that this driver uses, on Apple silicon. Other platforms use the
// driver-vbox-images repository, with its own version. The two are changed
// separately, because the two repositories do not always have the same
// releases.
const ImagesVersion = "0.4"

// ImagesSourceURL is the location where the master list of images can be found
var ImagesSourceURL = "https://github.com/kuttiproject/driver-vbox-arm64-images/releases/download/v" + ImagesVersion + "/" + imagesConfigFile
