package drivervbox

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// vboximagedata is a data-only representation of the Image type,
// used for serialization and output.
type vboximagedata struct {
	ImageK8sVersion string
	ImageChecksum   string
	ImageSourceURL  string
	ImageStatus     drivercore.ImageStatus
	ImageDeprecated bool
}

// Image implements the drivercore.Image interface for VirtualBox.
type Image struct {
	imageK8sVersion string
	imageChecksum   string
	imageSourceURL  string
	imageStatus     drivercore.ImageStatus
	imageDeprecated bool
}

// K8sVersion returns the version of Kubernetes present in the image.
func (i *Image) K8sVersion() string {
	return i.imageK8sVersion
}

// Status returns the status of the image.
// Status can be Downloaded, meaning the image exists in the local cache and can
// be used to create Machines, or Notdownloaded, meaning it has to be downloaded
// using Fetch.
func (i *Image) Status() drivercore.ImageStatus {
	return i.imageStatus
}

// Deprecated returns true if the image's version of Kubenetes is deprecated.
// New Machines should not be created from such an image.
func (i *Image) Deprecated() bool {
	return i.imageDeprecated
}

func (i *Image) fetch(progress func(int64, int64)) error {
	cachedir, err := vboxCacheDir()
	if err != nil {
		return err
	}

	tempfilename := fmt.Sprintf("kutti-k8s-%s.ovadownload", i.imageK8sVersion)
	tempfilepath := path.Join(cachedir, tempfilename)

	// Download file
	if progress != nil {
		err = workspace.DownloadFileWithProgress(i.imageSourceURL, tempfilepath, progress)
	} else {
		err = workspace.DownloadFile(i.imageSourceURL, tempfilepath)
	}
	if err != nil {
		return err
	}
	defer workspace.RemoveFile(tempfilepath)

	// Add
	return i.FromFile(tempfilepath)
}

// Fetch downloads the image from its source URL.
func (i *Image) Fetch() error {
	return i.fetch(nil)
	// cachedir, err := vboxCacheDir()
	// if err != nil {
	// 	return err
	// }

	// tempfilename := fmt.Sprintf("kutti-k8s-%s.ovadownload", i.imageK8sVersion)
	// tempfilepath := path.Join(cachedir, tempfilename)

	// // Download file
	// err = workspace.DownloadFile(i.imageSourceURL, tempfilepath)
	// if err != nil {
	// 	return err
	// }
	// defer workspace.RemoveFile(tempfilepath)

	// // Add
	// return i.FromFile(tempfilepath)
}

// FetchWithProgress downloads the image from the driver repository into the
// local cache, and reports progress via the supplied callback. The callback
// reports current and total in bytes.
func (i *Image) FetchWithProgress(progress func(current int64, total int64)) error {
	return i.fetch(progress)
}

// FromFile verifies an image file on a local path and copies it to the cache.
func (i *Image) FromFile(filepath string) error {
	err := addfromfile(i.imageK8sVersion, filepath, i.imageChecksum)
	if err != nil {
		return err
	}

	i.imageStatus = drivercore.ImageStatusDownloaded
	return imageconfigmanager.Save()
}

// PurgeLocal removes the local cached copy of an image.
func (i *Image) PurgeLocal() error {
	if i.imageStatus == drivercore.ImageStatusDownloaded {
		err := removefile(i.K8sVersion())
		if err == nil {
			i.imageStatus = drivercore.ImageStatusNotDownloaded

			return imageconfigmanager.Save()
		}
		return err
	}

	return nil
}

// MarshalJSON returns the JSON encoding of the image.
func (i *Image) MarshalJSON() ([]byte, error) {
	savedata := vboximagedata{
		ImageK8sVersion: i.imageK8sVersion,
		ImageChecksum:   i.imageChecksum,
		ImageSourceURL:  i.imageSourceURL,
		ImageStatus:     i.imageStatus,
		ImageDeprecated: i.imageDeprecated,
	}

	return json.Marshal(savedata)
}

// UnmarshalJSON  parses and restores a JSON-encoded
// image.
func (i *Image) UnmarshalJSON(b []byte) error {
	var loaddata vboximagedata

	err := json.Unmarshal(b, &loaddata)
	if err != nil {
		return err
	}

	i.imageK8sVersion = loaddata.ImageK8sVersion
	i.imageChecksum = loaddata.ImageChecksum
	i.imageSourceURL = loaddata.ImageSourceURL
	i.imageStatus = loaddata.ImageStatus
	i.imageDeprecated = loaddata.ImageDeprecated

	return nil
}
