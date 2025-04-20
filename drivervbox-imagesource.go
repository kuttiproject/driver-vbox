//go:build !darwin || !arm64

package drivervbox

// ImagesSourceURL is the location where the master list of images can be found
var ImagesSourceURL = "https://github.com/kuttiproject/driver-vbox-images/releases/download/v" + ImagesVersion + "/" + imagesConfigFile
