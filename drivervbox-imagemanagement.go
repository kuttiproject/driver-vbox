package drivervbox

import (
	"encoding/json"
	"errors"
	"path"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/kuttilog"
	"github.com/kuttiproject/workspace"
)

// ImagesVersion defines the image repository version for the current version
// of the driver.
const ImagesVersion = "0.4"

const imagesConfigFile = "driver-vbox-images.json"

var (
	imagedata             = &imageconfigdata{}
	imageconfigmanager, _ = workspace.NewFileConfigManager(imagesConfigFile, imagedata)
)

type imageconfigdata struct {
	images map[string]*Image
}

func (icd *imageconfigdata) Serialize() ([]byte, error) {
	return json.Marshal(icd.images)
}

func (icd *imageconfigdata) Deserialize(data []byte) error {
	loaddata := make(map[string]*Image)
	err := json.Unmarshal(data, &loaddata)
	if err == nil {
		icd.images = loaddata
	}
	return err
}

func (icd *imageconfigdata) SetDefaults() {
	icd.images = defaultimages()
}

func vboxCacheDir() (string, error) {
	return workspace.CacheSubDir("driver-vbox")
}

func vboxConfigDir() (string, error) {
	//return workspace.Configsubdir("vbox")
	return workspace.ConfigDir()
}

func defaultimages() map[string]*Image {
	return map[string]*Image{}
}

func imagenamefromk8sversion(k8sversion string) string {
	return "kutti-" + k8sversion + ".ova"
}

func imagepathfromk8sversion(k8sversion string) (string, error) {
	cachedir, err := vboxCacheDir()
	if err != nil {
		return "", err
	}

	result := path.Join(cachedir, imagenamefromk8sversion(k8sversion))
	return result, nil
}

func addfromfile(k8sversion string, filepath string, checksum string) error {
	kuttilog.Println(kuttilog.Info, "Checking image validity...")
	filechecksum, err := workspace.ChecksumFile(filepath)
	if err != nil {
		return err
	}

	if filechecksum != checksum {
		kuttilog.Printf(kuttilog.Debug, "checksum for file %v failed.\nWanted: %v\nGot   : %v\n", filepath, checksum, filechecksum)
		return errors.New("file  is not valid")
	}

	localfilepath, err := imagepathfromk8sversion(k8sversion)
	if err != nil {
		return err
	}

	kuttilog.Println(kuttilog.Info, "Copying image to local cache...")
	// A 128 KiB buffer should help
	const BUFSIZE = 131072
	err = workspace.CopyFile(filepath, localfilepath, BUFSIZE, true)
	if err != nil {
		return err
	}

	return nil
}

func removefile(k8sversion string) error {
	filename, err := imagepathfromk8sversion(k8sversion)
	if err != nil {
		return err
	}

	return workspace.RemoveFile(filename)
}

func fetchimagelist() error {
	// Download image list into temp directory
	confdir, _ := vboxConfigDir()
	tempfilename := "vboximagesnewlist.json"
	tempfilepath := path.Join(confdir, tempfilename)

	kuttilog.Printf(kuttilog.Debug, "confdir: %v\ntempfilepath: %v\n", confdir, tempfilepath)

	kuttilog.Println(kuttilog.Info, "Fetching image list...")
	kuttilog.Printf(kuttilog.Debug, "Fetching from %v into %v.", ImagesSourceURL, tempfilepath)
	err := workspace.DownloadFile(ImagesSourceURL, tempfilepath)
	kuttilog.Printf(kuttilog.Debug, "Error: %v", err)
	if err != nil {
		return err
	}
	defer workspace.RemoveFile(tempfilepath)

	// Load into object
	tempimagedata := &imageconfigdata{}
	tempconfigmanager, err := workspace.NewFileConfigManager(tempfilename, tempimagedata)
	if err != nil {
		return err
	}

	err = tempconfigmanager.Load()
	if err != nil {
		return err
	}

	// Compare against current and update
	for key, newimage := range tempimagedata.images {
		oldimage := imagedata.images[key]
		if oldimage != nil &&
			newimage.imageChecksum == oldimage.imageChecksum &&
			newimage.imageSourceURL == oldimage.imageSourceURL &&
			oldimage.imageStatus == drivercore.ImageStatusDownloaded {

			newimage.imageStatus = drivercore.ImageStatusDownloaded
		}
	}

	// Make it current
	imagedata.images = tempimagedata.images

	// Save as local configuration
	imageconfigmanager.Save()

	return nil
}
