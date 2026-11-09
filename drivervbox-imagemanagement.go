package drivervbox

import (
	"github.com/kuttiproject/imagecore"
	"github.com/kuttiproject/workspace"
)

const imagesConfigFile = "driver-vbox-images.json"

// imagestore manages the image list and the cached image files. See the
// imagecore package.
//
// It is made by init, and not in the declaration, so that it is made in the
// same way as in the other drivers, where the declaration is not possible.
// Making it does no I/O.
var imagestore *imagecore.Store

func init() {
	imagestore = imagecore.New(imagecore.Config{
		DriverName:     "driver-vbox",
		ConfigFile:     imagesConfigFile,
		ImageExtension: ".ova",
		DownloadSuffix: ".ovadownload",
		SourceURL:      func() string { return ImagesSourceURL },
		CacheDir:       vboxCacheDir,
	})
}

func vboxCacheDir() (string, error) {
	return workspace.CacheSubDir("driver-vbox")
}
