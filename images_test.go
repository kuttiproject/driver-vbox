package drivervbox

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuttiproject/drivercore"
	"github.com/kuttiproject/workspace"
)

// These tests use only what a user of the driver can use: the Driver methods
// that deal with images, and the Image methods. They check what the driver
// does with a list and image files from a local server, with the storage in
// temporary directories and a fake VBoxManage.
//
// The images of this driver are .ova files. The driver takes any file whose
// checksum is that of the image, and copies it to the cache.

type testimage struct {
	version    string
	content    []byte
	deprecated bool

	// path, if set, is where the server serves the file of the image, and
	// so what its source URL is. Otherwise it is "/kutti-<version>.ova".
	path string
}

func (ti testimage) urlpath() string {
	if ti.path != "" {
		return ti.path
	}

	return "/kutti-" + ti.version + ".ova"
}

func (ti testimage) checksum() string {
	return fmt.Sprintf("%x", sha256.Sum256(ti.content))
}

// content makes content for a test image that is different for different
// seeds.
func content(seed string, size int) []byte {
	result := make([]byte, size)
	for i := range result {
		result[i] = seed[i%len(seed)] + byte(i%251)
	}

	return result
}

// imageserver serves an image list, and the images in it. A test can change
// what it serves.
type imageserver struct {
	*httptest.Server

	mu       sync.Mutex
	images   []testimage
	listcode int
	rawlist  string
	delay    time.Duration
	files    map[string][]byte
	requests map[string]int
}

func newImageServer(t *testing.T, images ...testimage) *imageserver {
	t.Helper()

	is := &imageserver{
		listcode: http.StatusOK,
		files:    map[string][]byte{},
		requests: map[string]int{},
	}
	is.set(images...)

	is.Server = httptest.NewServer(http.HandlerFunc(is.handle))
	t.Cleanup(is.Close)

	saved := ImagesSourceURL
	ImagesSourceURL = is.URL + "/" + imagesConfigFile
	t.Cleanup(func() { ImagesSourceURL = saved })

	return is
}

// set changes the images in the list. The files served are the contents of
// the images.
func (is *imageserver) set(images ...testimage) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.images = images
	for _, img := range images {
		is.files[img.urlpath()] = img.content
	}
}

// serveFile changes what is served as the file of an image, without changing
// the list.
func (is *imageserver) serveFile(version string, data []byte) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.files["/kutti-"+version+".ova"] = data
}

// dropFile makes the server stop having the file of an image.
func (is *imageserver) dropFile(version string) {
	is.mu.Lock()
	defer is.mu.Unlock()

	delete(is.files, "/kutti-"+version+".ova")
}

func (is *imageserver) failList(code int) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.listcode = code
}

// fileRequests returns how many times the file of an image was requested.
func (is *imageserver) fileRequests(version string) int {
	is.mu.Lock()
	defer is.mu.Unlock()

	return is.requests["/kutti-"+version+".ova"]
}

// serveRawList makes the server answer requests for the list with the given
// text, whatever the images are.
func (is *imageserver) serveRawList(body string) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.rawlist = body
}

// slowDownFiles makes the server wait before it answers a request for an
// image file.
func (is *imageserver) slowDownFiles(delay time.Duration) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.delay = delay
}

func (is *imageserver) handle(w http.ResponseWriter, r *http.Request) {
	is.mu.Lock()
	delay := is.delay
	is.requests[r.URL.Path]++
	is.mu.Unlock()

	if r.URL.Path != "/"+imagesConfigFile {
		time.Sleep(delay)
	}

	is.mu.Lock()
	defer is.mu.Unlock()

	if r.URL.Path == "/"+imagesConfigFile {
		if is.listcode != http.StatusOK {
			http.Error(w, "no list", is.listcode)
			return
		}
		if is.rawlist != "" {
			w.Write([]byte(is.rawlist))
			return
		}

		list := map[string]map[string]interface{}{}
		for _, img := range is.images {
			list[img.version] = map[string]interface{}{
				"ImageK8sVersion": img.version,
				"ImageChecksum":   img.checksum(),
				"ImageSourceURL":  "http://" + r.Host + img.urlpath(),
				"ImageStatus":     "NotDownloaded",
				"ImageDeprecated": img.deprecated,
			}
		}
		json.NewEncoder(w).Encode(list)
		return
	}

	data, ok := is.files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.Write(data)
}

func getImage(t *testing.T, version string) drivercore.Image {
	t.Helper()

	img, err := (&Driver{}).GetImage(version)
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func isDownloaded(t *testing.T, version string) bool {
	t.Helper()
	return getImage(t, version).Status() == drivercore.ImageStatusDownloaded
}

// updateList gets the list from the server.
func updateList(t *testing.T) {
	t.Helper()

	if err := (&Driver{}).UpdateImageList(); err != nil {
		t.Fatalf("UpdateImageList failed: %v", err)
	}
}

// listEntry reads the saved list, and returns the entry of one version.
func listEntry(t *testing.T, version string) map[string]interface{} {
	t.Helper()

	dir, err := workspace.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	data := readFile(t, filepath.Join(dir, imagesConfigFile))

	var list map[string]map[string]interface{}
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}

	return list[version]
}

// statusInListFile reads the status of an image from the saved list.
func statusInListFile(t *testing.T, version string) string {
	t.Helper()

	status, _ := listEntry(t, version)["ImageStatus"].(string)
	return status
}

func TestImageListQueries(t *testing.T) {
	testStorage(t)
	newImageServer(t,
		testimage{version: "1.36", content: content("a", 100), deprecated: true},
		testimage{version: "1.37", content: content("b", 100), deprecated: false},
	)
	updateList(t)

	d := &Driver{}

	versions := d.K8sVersions()
	sort.Strings(versions)
	if strings.Join(versions, ",") != "1.36,1.37" {
		t.Errorf("versions are %v", versions)
	}

	if !d.ValidK8sVersion("1.37") || d.ValidK8sVersion("1.99") {
		t.Error("ValidK8sVersion gave the wrong answer")
	}

	images, err := d.ListImages()
	if err != nil || len(images) != 2 {
		t.Fatalf("ListImages returned %d images, %v", len(images), err)
	}

	old := getImage(t, "1.36")
	if old.K8sVersion() != "1.36" || !old.Deprecated() {
		t.Errorf("image 1.36 is %q, deprecated %v", old.K8sVersion(), old.Deprecated())
	}
	current := getImage(t, "1.37")
	if current.K8sVersion() != "1.37" || current.Deprecated() {
		t.Errorf("image 1.37 is %q, deprecated %v", current.K8sVersion(), current.Deprecated())
	}
	if current.Status() == drivercore.ImageStatusDownloaded {
		t.Error("a new image is marked as downloaded")
	}
}

func TestGetImageOfUnknownVersionGivesError(t *testing.T) {
	testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 100)})
	updateList(t)

	_, err := (&Driver{}).GetImage("1.99")
	if err == nil {
		t.Fatal("GetImage of an unknown version succeeded")
	}
	if !strings.Contains(err.Error(), "1.99") {
		t.Errorf("the error does not name the version: %v", err)
	}
}

func TestNoImagesBeforeTheListIsUpdated(t *testing.T) {
	testStorage(t)
	d := &Driver{}

	if versions := d.K8sVersions(); len(versions) != 0 {
		t.Errorf("versions are %v", versions)
	}
	if d.ValidK8sVersion("1.37") {
		t.Error("a version is valid before the list is updated")
	}
	if images, err := d.ListImages(); err != nil || len(images) != 0 {
		t.Errorf("ListImages returned %d images, %v", len(images), err)
	}
}

func TestUpdateImageListFailureKeepsList(t *testing.T) {
	testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 100)})
	updateList(t)

	server.failList(http.StatusNotFound)
	if err := (&Driver{}).UpdateImageList(); err == nil {
		t.Error("UpdateImageList succeeded although the server failed")
	}

	if versions := (&Driver{}).K8sVersions(); len(versions) != 1 || versions[0] != "1.37" {
		t.Errorf("versions are %v", versions)
	}
}

func TestImageListIsKeptInTheConfigDirectory(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 100), deprecated: true})
	updateList(t)

	// The format of the file is part of what users have on their disks.
	var list map[string]map[string]interface{}
	if err := json.Unmarshal(readFile(t, listFilePath(root)), &list); err != nil {
		t.Fatal(err)
	}

	entry := list["1.37"]
	keys := []string{}
	for key := range entry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := "ImageChecksum,ImageDeprecated,ImageK8sVersion,ImageSourceURL,ImageStatus"
	if strings.Join(keys, ",") != want {
		t.Errorf("the entry has the keys %v", keys)
	}
	if entry["ImageK8sVersion"] != "1.37" || entry["ImageDeprecated"] != true {
		t.Errorf("the entry is %v", entry)
	}
	if !strings.HasSuffix(fmt.Sprint(entry["ImageSourceURL"]), "/kutti-1.37.ova") {
		t.Errorf("the source URL is %v", entry["ImageSourceURL"])
	}
}

// Updating the list downloads it into a file of its own. Whatever the driver
// keeps next to the list, the file it downloaded into must not stay.
func TestUpdateImageListLeavesNoDownloadInTheConfigDirectory(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 100)})

	check := func(when string) {
		t.Helper()

		entries, err := os.ReadDir(filepath.Dir(listFilePath(root)))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "driver-vbox-images.json" && !strings.HasSuffix(e.Name(), ".lock") {
				t.Errorf("%s the config directory holds %q", when, e.Name())
			}
		}
	}

	updateList(t)
	check("after an update,")

	server.failList(http.StatusInternalServerError)
	if err := (&Driver{}).UpdateImageList(); err == nil {
		t.Fatal("UpdateImageList succeeded although the server failed")
	}
	check("after a failed update,")
}

func TestFetch(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 3<<20)}
	newImageServer(t, image)
	updateList(t)

	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if string(readFile(t, cachedImagePath(root, "1.37"))) != string(image.content) {
		t.Error("the cached file does not hold the image")
	}
	if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.37.ova" {
		t.Errorf("the cache directory holds %v", names)
	}

	if !isDownloaded(t, "1.37") {
		t.Error("the image is not marked as downloaded")
	}
	if status := statusInListFile(t, "1.37"); status != string(drivercore.ImageStatusDownloaded) {
		t.Errorf("the saved list has status %q", status)
	}
}

func TestFetchWithProgress(t *testing.T) {
	testStorage(t)
	image := testimage{version: "1.37", content: content("b", 3<<20)}
	newImageServer(t, image)
	updateList(t)

	var calls int
	var last, total int64
	err := getImage(t, "1.37").FetchWithProgress(func(current int64, size int64) {
		calls++
		last, total = current, size
	})
	if err != nil {
		t.Fatal(err)
	}

	size := int64(len(image.content))
	if calls == 0 || last != size || total != last {
		t.Errorf("progress was called %d times, ending at %d of %d, the image is %d bytes", calls, last, total, size)
	}
}

func TestFetchOfAnImageThatIsDownloadedDownloadsAgain(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 5000)}
	server := newImageServer(t, image)
	updateList(t)

	for i := 1; i <= 2; i++ {
		if err := getImage(t, "1.37").Fetch(); err != nil {
			t.Fatalf("Fetch %d failed: %v", i, err)
		}
		if got := server.fileRequests("1.37"); got != i {
			t.Errorf("after %d fetches the image was requested %d times", i, got)
		}
	}

	if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.37.ova" {
		t.Errorf("the cache directory holds %v", names)
	}
}

func TestFetchOfCorruptImageKeepsNothing(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 1000)})
	updateList(t)

	server.serveFile("1.37", content("x", 1000))

	err := getImage(t, "1.37").Fetch()
	if err == nil || !strings.Contains(err.Error(), "is not valid") {
		t.Fatalf("Fetch returned %v, expected an error that the file is not valid", err)
	}

	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
	if isDownloaded(t, "1.37") {
		t.Error("the image is marked as downloaded")
	}
}

func TestFetchWhenTheServerFailsKeepsNothing(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 1000)})
	updateList(t)

	server.dropFile("1.37")

	if err := getImage(t, "1.37").Fetch(); err == nil {
		t.Fatal("Fetch succeeded although the server has no file")
	}
	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
	if isDownloaded(t, "1.37") {
		t.Error("the image is marked as downloaded")
	}
}

// The driver checks only the checksum of a file, so a file may have any name,
// or none that looks like an image.
func TestFromFileTakesAnyFileWithTheRightChecksum(t *testing.T) {
	for _, name := range []string{"mine.ova", "mine.bin", "mine", "MINE.OVA", "my image.ova"} {
		t.Run(name, func(t *testing.T) {
			root := testStorage(t)
			image := testimage{version: "1.37", content: content("b", 5000)}
			newImageServer(t, image)
			updateList(t)

			source := filepath.Join(t.TempDir(), name)
			writeFile(t, source, image.content)

			if err := getImage(t, "1.37").FromFile(source); err != nil {
				t.Fatalf("FromFile failed: %v", err)
			}

			if string(readFile(t, cachedImagePath(root, "1.37"))) != string(image.content) {
				t.Error("the cached file does not hold the image")
			}
			if !isDownloaded(t, "1.37") {
				t.Error("the image is not marked as downloaded")
			}
			if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.37.ova" {
				t.Errorf("the cache directory holds %v", names)
			}

			// The file of the user is theirs, and stays.
			if string(readFile(t, source)) != string(image.content) {
				t.Error("the source file was changed or removed")
			}
		})
	}
}

func TestFromFileRejectsWrongFile(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 5000)})
	updateList(t)

	source := filepath.Join(t.TempDir(), "mine.ova")
	writeFile(t, source, content("x", 5000))

	err := getImage(t, "1.37").FromFile(source)
	if err == nil || !strings.Contains(err.Error(), "is not valid") {
		t.Fatalf("FromFile returned %v, expected an error that the file is not valid", err)
	}
	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
	if isDownloaded(t, "1.37") {
		t.Error("the image is marked as downloaded")
	}
}

func TestFromFileOfFilesThatCannotBeUsed(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{"a file that does not exist", func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.ova") }},
		{"a directory", func(t *testing.T) string { return t.TempDir() }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testStorage(t)
			newImageServer(t, testimage{version: "1.37", content: content("b", 5000)})
			updateList(t)

			if err := getImage(t, "1.37").FromFile(tc.path(t)); err == nil {
				t.Fatal("FromFile succeeded")
			}
			if names := imageFiles(t, root); len(names) != 0 {
				t.Errorf("the cache directory holds %v", names)
			}
			if isDownloaded(t, "1.37") {
				t.Error("the image is marked as downloaded")
			}
		})
	}
}

// Vbox does not look at the file in the cache, or at what uses it, because
// every machine gets its own copy of the image when it is imported. So a
// file that has the right checksum is always copied.
func TestFromFileCopiesAgainWhenTheSameImageIsCached(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 5000)}
	newImageServer(t, image)
	updateList(t)

	source := filepath.Join(t.TempDir(), "mine.ova")
	writeFile(t, source, image.content)
	if err := getImage(t, "1.37").FromFile(source); err != nil {
		t.Fatal(err)
	}

	cached := cachedImagePath(root, "1.37")
	longago := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(cached, longago, longago); err != nil {
		t.Fatal(err)
	}

	if err := getImage(t, "1.37").FromFile(source); err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	info, err := os.Stat(cached)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Before(time.Now().Add(-time.Hour)) {
		t.Error("the cached file was not written again")
	}
}

// publishNew makes the server publish different content for a version, and
// updates the list. The status of the image becomes not downloaded, because
// its checksum is no longer that of the cached file.
func publishNew(t *testing.T, server *imageserver, version string, data []byte) string {
	t.Helper()

	server.set(testimage{version: version, content: data})
	updateList(t)

	source := filepath.Join(t.TempDir(), "new.ova")
	writeFile(t, source, data)
	return source
}

func TestFromFileReplacesAnOldImage(t *testing.T) {
	root := testStorage(t)
	old := testimage{version: "1.37", content: content("b", 5000)}
	server := newImageServer(t, old)
	updateList(t)
	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatal(err)
	}

	newcontent := content("n", 6000)
	source := publishNew(t, server, "1.37", newcontent)
	if isDownloaded(t, "1.37") {
		t.Fatal("the new image is marked as downloaded before it is")
	}

	if err := getImage(t, "1.37").FromFile(source); err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	if string(readFile(t, cachedImagePath(root, "1.37"))) != string(newcontent) {
		t.Error("the cached file is not the new image")
	}
	if !isDownloaded(t, "1.37") {
		t.Error("the new image is not marked as downloaded")
	}
}

func TestPurgeLocal(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.36", content: content("a", 3000)}, testimage{version: "1.37", content: content("b", 4000)})
	updateList(t)
	for _, v := range []string{"1.36", "1.37"} {
		if err := getImage(t, v).Fetch(); err != nil {
			t.Fatal(err)
		}
	}

	if err := getImage(t, "1.37").PurgeLocal(); err != nil {
		t.Fatalf("PurgeLocal failed: %v", err)
	}

	if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.36.ova" {
		t.Errorf("the cache directory holds %v", names)
	}
	if isDownloaded(t, "1.37") || !isDownloaded(t, "1.36") {
		t.Error("the status of the images is wrong")
	}
	if status := statusInListFile(t, "1.37"); status != string(drivercore.ImageStatusNotDownloaded) {
		t.Errorf("the saved list has status %q for 1.37", status)
	}
	if status := statusInListFile(t, "1.36"); status != string(drivercore.ImageStatusDownloaded) {
		t.Errorf("the saved list has status %q for 1.36", status)
	}
}

func TestPurgeLocalOfAnImageThatIsNotDownloadedDoesNothing(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 4000)})
	updateList(t)

	if err := getImage(t, "1.37").PurgeLocal(); err != nil {
		t.Fatalf("PurgeLocal failed: %v", err)
	}
	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
}

func TestUpdateImageListKeepsDownloadedStatusOfUnchangedImage(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t,
		testimage{version: "1.36", content: content("a", 3000)},
		testimage{version: "1.37", content: content("b", 4000)},
	)
	updateList(t)
	if err := getImage(t, "1.36").Fetch(); err != nil {
		t.Fatal(err)
	}

	// The same images, and a new one, and a change that is not to the image.
	server.set(
		testimage{version: "1.36", content: content("a", 3000), deprecated: true},
		testimage{version: "1.37", content: content("b", 4000)},
		testimage{version: "1.38", content: content("c", 2000)},
	)
	updateList(t)

	if !isDownloaded(t, "1.36") {
		t.Error("the image that did not change lost its downloaded status")
	}
	if !getImage(t, "1.36").Deprecated() {
		t.Error("the image does not show that it is now deprecated")
	}
	if isDownloaded(t, "1.37") || isDownloaded(t, "1.38") {
		t.Error("an image that was not downloaded is marked as downloaded")
	}
	if status := statusInListFile(t, "1.36"); status != string(drivercore.ImageStatusDownloaded) {
		t.Errorf("the saved list has status %q for 1.36", status)
	}
	if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.36.ova" {
		t.Errorf("the cache directory holds %v", names)
	}
}

func TestUpdateImageListDropsDownloadedStatusOfChangedImage(t *testing.T) {
	cases := []struct {
		name   string
		change func(server *imageserver)
	}{
		{"checksum", func(server *imageserver) {
			server.set(testimage{version: "1.37", content: content("changed", 4000)})
		}},
		{"source URL", func(server *imageserver) {
			server.set(testimage{version: "1.37", content: content("b", 4000), path: "/elsewhere/kutti-1.37.ova"})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testStorage(t)
			server := newImageServer(t, testimage{version: "1.37", content: content("b", 4000)})
			updateList(t)
			if err := getImage(t, "1.37").Fetch(); err != nil {
				t.Fatal(err)
			}

			tc.change(server)
			updateList(t)

			if isDownloaded(t, "1.37") {
				t.Error("the image is still marked as downloaded")
			}

			// Only the list is changed. The file stays, until it is purged
			// or replaced.
			if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.37.ova" {
				t.Errorf("the cache directory holds %v", names)
			}
		})
	}
}

// Machines are made from the cached image, so the driver must look for it
// where it has always been, and give the path in a way VBoxManage takes.
func TestNewMachineImportsTheCachedImage(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 5000)}
	newImageServer(t, image)
	updateList(t)
	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatal(err)
	}

	// The fake makes VBoxManage fail to import. That is where this test
	// wants NewMachine to stop.
	machine, err := (&Driver{}).NewMachine("node1", "cluster1", "1.37")
	if err == nil || !strings.Contains(err.Error(), "could not import ovafile") {
		t.Fatalf("NewMachine returned %v, expected an error that the import failed", err)
	}
	if machine != nil {
		t.Errorf("NewMachine returned a machine, %v", machine)
	}

	calls := vbmCalls(t)
	if len(calls) != 2 || calls[0] != "--version" {
		t.Fatalf("the calls to VBoxManage were %q", calls)
	}

	// The path of the image is compared as the platform writes it: a path
	// made with the path package has slashes on Windows.
	fields := strings.Fields(calls[1])
	if len(fields) < 2 || fields[0] != "import" {
		t.Fatalf("the second call to VBoxManage was %q", calls[1])
	}
	if filepath.ToSlash(fields[1]) != filepath.ToSlash(cachedImagePath(root, "1.37")) {
		t.Errorf("the image to import is %q, the cached image is %q", fields[1], cachedImagePath(root, "1.37"))
	}

	rest := strings.Join(fields[2:], " ")
	want := "--vsys 0 --vmname cluster1-node1 --vsys 0 --group /cluster1 --vsys 0 --basefolder " + machinesDirPath(root)
	if rest != want {
		t.Errorf("the rest of the import command is\n%s\nwanted\n%s", rest, want)
	}
}

func TestNewMachineWithoutTheImageFails(t *testing.T) {
	testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 5000)})
	updateList(t)

	machine, err := (&Driver{}).NewMachine("node1", "cluster1", "1.37")
	if err == nil || !strings.Contains(err.Error(), "could not retrieve image") {
		t.Fatalf("NewMachine returned %v, expected an error that the image is missing", err)
	}
	if machine != nil {
		t.Errorf("NewMachine returned a machine, %v", machine)
	}

	// VBoxManage was asked only for its version.
	if calls := vbmCalls(t); len(calls) != 1 || calls[0] != "--version" {
		t.Errorf("the calls to VBoxManage were %q", calls)
	}
}
