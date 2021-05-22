package drivervbox

import (
	"fmt"
	"path"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// Image implements the drivercore.Image interface for VirtualBox.
type Image struct {
	ImageK8sVersion string
	ImageChecksum   string
	ImageSourceURL  string
	ImageStatus     drivercore.ImageStatus
	ImageDeprecated bool
}

// K8sVersion returns the version of Kubernetes present in the image.
func (i *Image) K8sVersion() string {
	return i.ImageK8sVersion
}

// Status returns the status of the image.
// Status can be Downloaded, meaning the image exists in the local cache and can
// be used to create Machines, or Notdownloaded, meaning it has to be downloaded
// using Fetch.
func (i *Image) Status() drivercore.ImageStatus {
	return i.ImageStatus
}

// Deprecated returns true if the image's version of Kubenetes is deprecated.
// New Macines should not be created from such an image.
func (i *Image) Deprecated() bool {
	return i.ImageDeprecated
}

// Fetch downloads the image from its source URL.
func (i *Image) Fetch() error {
	cachedir, err := vboxCacheDir()
	if err != nil {
		return err
	}

	tempfilename := fmt.Sprintf("kutti-k8s-%s.ovadownload", i.ImageK8sVersion)
	tempfilepath := path.Join(cachedir, tempfilename)

	// Download file
	err = workspace.DownloadFile(i.ImageSourceURL, tempfilepath)
	if err != nil {
		return err
	}
	defer workspace.RemoveFile(tempfilepath)

	// Add
	return i.FromFile(tempfilepath)
}

// FromFile verifies an image file on a local path and copies it to the cache.
func (i *Image) FromFile(filepath string) error {
	err := addfromfile(i.ImageK8sVersion, filepath, i.ImageChecksum)
	if err != nil {
		return err
	}

	i.ImageStatus = drivercore.ImageStatusDownloaded
	return imageconfigmanager.Save()
}

// PurgeLocal removes the local cached copy of an image.
func (i *Image) PurgeLocal() error {
	if i.ImageStatus == drivercore.ImageStatusDownloaded {
		err := removefile(i.K8sVersion())
		if err == nil {
			i.ImageStatus = drivercore.ImageStatusNotDownloaded

			return imageconfigmanager.Save()
		}
		return err
	}

	return nil
}
