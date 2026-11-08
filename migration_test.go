package drivervbox

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuttiproject/drivercore"
)

// These tests are for what changed when the image code moved to imagecore.
// What did not change is in images_test.go, which passed before the move, and
// still does.

// rawEntry is an entry in a list that is not made by the server.
func rawEntry(version string, key string, checksum string, status string) string {
	return fmt.Sprintf(
		`%q:{"ImageK8sVersion":%q,"ImageChecksum":%q,"ImageSourceURL":"http://localhost/kutti-%s.ova","ImageStatus":%q,"ImageDeprecated":false}`,
		key, version, checksum, version, status,
	)
}

// A downloaded list that cannot be used is refused as a whole, and the list
// in use stays as it was.
func TestUpdateImageListWithListThatCannotBeUsedKeepsList(t *testing.T) {
	good := content("b", 100)
	sum := testimage{content: good}.checksum()

	cases := []struct {
		name string
		body string
	}{
		{"not JSON", "this is not a list"},
		{"a list with a null entry", `{"1.38":null}`},
		{"no images", `{}`},
		{"null", `null`},
		{"a version that is a path", "{" + rawEntry("../x", "../x", sum, "NotDownloaded") + "}"},
		{"a key that is not the version of the entry", "{" + rawEntry("1.38", "1.39", sum, "NotDownloaded") + "}"},
		{"one bad entry among good ones", "{" + rawEntry("1.38", "1.38", sum, "NotDownloaded") + "," + rawEntry("a/b", "a/b", sum, "NotDownloaded") + "}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testStorage(t)
			server := newImageServer(t, testimage{version: "1.37", content: good})
			updateList(t)
			before := readFile(t, listFilePath(root))

			server.serveRawList(tc.body)
			if err := (&Driver{}).UpdateImageList(); err == nil {
				t.Fatal("UpdateImageList succeeded")
			}

			if versions := (&Driver{}).K8sVersions(); len(versions) != 1 || versions[0] != "1.37" {
				t.Errorf("versions are %v", versions)
			}
			if string(readFile(t, listFilePath(root))) != string(before) {
				t.Error("the list file was changed")
			}

			entries, err := os.ReadDir(filepath.Dir(listFilePath(root)))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".corrupt") {
					t.Errorf("a file was set aside: %s", e.Name())
				}
			}
		})
	}
}

// The status of an image in the list is what the driver knows about the
// cache, so a published list cannot say it.
func TestUpdateImageListIgnoresTheStatusInThePublishedList(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 100)})

	sum := testimage{content: content("b", 100)}.checksum()
	server.serveRawList("{" + rawEntry("1.37", "1.37", sum, "Downloaded") + "}")
	updateList(t)

	if isDownloaded(t, "1.37") {
		t.Error("the image is marked as downloaded, but there is no file")
	}
	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
}

func TestImagesAreListedInVersionOrder(t *testing.T) {
	testStorage(t)
	newImageServer(t,
		testimage{version: "1.37", content: content("a", 100)},
		testimage{version: "1.9", content: content("b", 100)},
		testimage{version: "1.10", content: content("c", 100)},
		testimage{version: "1.2", content: content("d", 100)},
	)
	updateList(t)

	d := &Driver{}
	if got := strings.Join(d.K8sVersions(), ","); got != "1.2,1.9,1.10,1.37" {
		t.Errorf("K8sVersions gave %s", got)
	}

	images, err := d.ListImages()
	if err != nil {
		t.Fatal(err)
	}
	versions := []string{}
	for _, img := range images {
		versions = append(versions, img.K8sVersion())
	}
	if got := strings.Join(versions, ","); got != "1.2,1.9,1.10,1.37" {
		t.Errorf("ListImages gave %s", got)
	}
}

// What GetImage returns for an unknown version must be nil, so that a caller
// can compare it to nil.
func TestGetImageOfUnknownVersionGivesNoImage(t *testing.T) {
	testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 100)})
	updateList(t)

	img, err := (&Driver{}).GetImage("1.99")
	if err == nil {
		t.Fatal("GetImage succeeded")
	}
	if img != nil {
		t.Errorf("GetImage returned an image, %#v", img)
	}
}

// A list file on disk that cannot be used is left alone by anything that only
// reads, and is repaired by an update, which keeps the damaged file.
func TestDamagedLocalListIsReportedAndRepairedByUpdate(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 100)})
	updateList(t)

	damaged := []byte(`{"1.37": "not an image"}`)
	writeFile(t, listFilePath(root), damaged)

	d := &Driver{}
	_, err := d.ListImages()
	if err == nil || !strings.Contains(err.Error(), "driver-vbox-images.json") {
		t.Errorf("ListImages returned %v, expected an error that names the file", err)
	}
	if d.ValidK8sVersion("1.37") {
		t.Error("a version is valid although the list cannot be used")
	}
	if string(readFile(t, listFilePath(root))) != string(damaged) {
		t.Error("reading changed the list file")
	}

	updateList(t)
	if versions := d.K8sVersions(); len(versions) != 1 || versions[0] != "1.37" {
		t.Errorf("versions are %v", versions)
	}

	var aside []string
	entries, err := os.ReadDir(filepath.Dir(listFilePath(root)))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".corrupt") {
			aside = append(aside, e.Name())
		}
	}
	if len(aside) != 1 || !strings.HasPrefix(aside[0], "driver-vbox-images.json.") {
		t.Fatalf("the files set aside are %v", aside)
	}
	if string(readFile(t, filepath.Join(filepath.Dir(listFilePath(root)), aside[0]))) != string(damaged) {
		t.Error("the damaged file was not kept as it was")
	}
}

// Purging succeeds when the cached file is already gone, and puts right what
// the list says.
func TestPurgeLocalWhenTheCachedFileIsGone(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 4000)})
	updateList(t)
	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(cachedImagePath(root, "1.37")); err != nil {
		t.Fatal(err)
	}

	if err := getImage(t, "1.37").PurgeLocal(); err != nil {
		t.Fatalf("PurgeLocal failed: %v", err)
	}
	if isDownloaded(t, "1.37") {
		t.Error("the image is still marked as downloaded")
	}
}

// An image that is downloaded, with its file gone, is downloaded again.
func TestFetchRepairsAMissingCachedFile(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 4000)}
	newImageServer(t, image)
	updateList(t)
	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cachedImagePath(root, "1.37")); err != nil {
		t.Fatal(err)
	}

	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if string(readFile(t, cachedImagePath(root, "1.37"))) != string(image.content) {
		t.Error("the cached file does not hold the image")
	}
}

// An Image was made from an entry in the list. When the list changes, an old
// Image does not put a file in the cache that the new list does not describe.
func TestImageObtainedBeforeAListUpdateIsRefusedAfterIt(t *testing.T) {
	root := testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 4000)})
	updateList(t)
	old := getImage(t, "1.37")

	server.set(testimage{version: "1.37", content: content("changed", 4000)})
	updateList(t)

	err := old.Fetch()
	if err == nil || !strings.Contains(err.Error(), "has changed or been removed in the image list") {
		t.Fatalf("Fetch returned %v", err)
	}
	if names := imageFiles(t, root); len(names) != 0 {
		t.Errorf("the cache directory holds %v", names)
	}
}

// Two processes that fetch the same image take turns, and the second finds
// the work done.
func TestFetchesOfOneImageAreSerialized(t *testing.T) {
	root := testStorage(t)
	image := testimage{version: "1.37", content: content("b", 1<<20)}
	server := newImageServer(t, image)
	updateList(t)
	server.slowDownFiles(300 * time.Millisecond)

	first, second := getImage(t, "1.37"), getImage(t, "1.37")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, img := range []drivercore.Image{first, second} {
		wg.Add(1)
		go func(i int, img drivercore.Image) {
			defer wg.Done()
			errs[i] = img.Fetch()
		}(i, img)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("fetch %d failed: %v", i, err)
		}
	}
	if hits := server.fileRequests("1.37"); hits != 1 {
		t.Errorf("the image was downloaded %d times", hits)
	}
	if string(readFile(t, cachedImagePath(root, "1.37"))) != string(image.content) {
		t.Error("the cached file does not hold the image")
	}
	if names := imageFiles(t, root); strings.Join(names, ",") != "kutti-1.37.ova" {
		t.Errorf("the cache directory holds %v", names)
	}
}

// The files that the driver keeps in the config directory.
func TestFilesInTheConfigDirectory(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 1000)})
	updateList(t)
	if err := getImage(t, "1.37").Fetch(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Dir(listFilePath(root)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	expected := "driver-vbox-images-1.37.lock,driver-vbox-images.json,driver-vbox-images.lock"
	if got := strings.Join(names, ","); got != expected {
		t.Errorf("the config directory holds %s, expected %s", got, expected)
	}
}

// An image is downloaded into a work directory of the cache directory, so
// that what is in progress is never taken for an image.
func TestFetchDownloadsIntoAWorkDirInTheCache(t *testing.T) {
	root := testStorage(t)
	newImageServer(t, testimage{version: "1.37", content: content("b", 3<<20)})
	updateList(t)

	cache := cacheDirPath(root)
	var found []string
	err := getImage(t, "1.37").FetchWithProgress(func(current int64, total int64) {
		if found != nil {
			return
		}

		filepath.WalkDir(cache, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(cache, path)
				found = append(found, rel)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Fatalf("during the download the cache directory held %v", found)
	}
	dir, name := filepath.Split(found[0])
	if dir == "" || !strings.HasPrefix(name, "kutti-k8s-1.37.ovadownload") {
		t.Errorf("the image was downloaded into %s", found[0])
	}
	if !strings.HasPrefix(dir, "kutti-k8s-1.37.") || !strings.HasSuffix(filepath.Clean(dir), ".work") {
		t.Errorf("the work directory is %s", dir)
	}
}

// The error is the same for a download and for a file of the user.
func TestMessageThatTheFileIsNotValid(t *testing.T) {
	testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 1000)})
	updateList(t)

	server.serveFile("1.37", content("x", 1000))
	if err := getImage(t, "1.37").Fetch(); err == nil || err.Error() != "file is not valid" {
		t.Errorf("Fetch returned %v", err)
	}

	source := filepath.Join(t.TempDir(), "mine.ova")
	writeFile(t, source, content("x", 1000))
	if err := getImage(t, "1.37").FromFile(source); err == nil || err.Error() != "file is not valid" {
		t.Errorf("FromFile returned %v", err)
	}
}

// A downloaded list may not be so large that it is a problem.
func TestUpdateImageListRefusesAListThatIsTooLarge(t *testing.T) {
	testStorage(t)
	server := newImageServer(t, testimage{version: "1.37", content: content("b", 100)})
	updateList(t)

	sum := testimage{content: content("b", 100)}.checksum()
	server.serveRawList("{" + rawEntry("1.37", "1.37", sum, "NotDownloaded") + "}" + strings.Repeat(" ", 5<<20))
	if err := (&Driver{}).UpdateImageList(); err == nil {
		t.Error("UpdateImageList accepted a list of more than 4 MiB")
	}
}

// What is not a version is not made into a path.
func TestNewMachineRefusesAVersionThatIsNotAVersion(t *testing.T) {
	testStorage(t)

	for _, version := range []string{"../1.37", "1.37/../../x", ""} {
		machine, err := (&Driver{}).NewMachine("node1", "cluster1", version)
		if err == nil || machine != nil {
			t.Errorf("version %q: NewMachine returned %v, %v", version, machine, err)
		}
		if err != nil && strings.Contains(err.Error(), "could not retrieve image") {
			t.Errorf("version %q was made into a path: %v", version, err)
		}
	}

	// It stopped before it made anything.
	for _, call := range vbmCalls(t) {
		if call != "--version" {
			t.Errorf("unexpected call to VBoxManage: %s", call)
		}
	}
}
