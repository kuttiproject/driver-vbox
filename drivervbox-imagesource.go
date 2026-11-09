//go:build !darwin || !arm64

package drivervbox

// ImagesVersion is the version of the driver-vbox-images repository that this
// driver uses, on every platform except Apple silicon. Apple silicon has its
// own images repository, with its own version.
const ImagesVersion = "0.5"

// ImagesSourceURL is the location where the master list of images can be found
var ImagesSourceURL = "https://github.com/kuttiproject/driver-vbox-images/releases/download/v" + ImagesVersion + "/" + imagesConfigFile
