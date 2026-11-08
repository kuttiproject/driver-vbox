package drivervbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kuttiproject/kuttilog"
	"github.com/kuttiproject/workspace"
)

// quietlogger is a kuttilog logger that discards everything, to keep the
// output of the driver off the output of the tests.
type quietlogger struct{}

func (quietlogger) MaxLevel() int                     { return kuttilog.Debug }
func (quietlogger) LevelPrefix(level int) string      { return "" }
func (quietlogger) Print(v ...interface{})            {}
func (quietlogger) Printf(f string, v ...interface{}) {}
func (quietlogger) Println(v ...interface{})          {}

// The tests run the driver against a fake VBoxManage, which is the test
// binary itself. TestMain turns the binary into the fake when
// fakeVBoxManageEnv is set, which the tests do, through testStorage, only for
// the processes they start. That works the same on every platform, and needs
// no VirtualBox.
const fakeVBoxManageEnv = "KUTTI_TEST_FAKE_VBOXMANAGE"

func TestMain(m *testing.M) {
	if os.Getenv(fakeVBoxManageEnv) != "" {
		os.Exit(fakeVBoxManage(os.Args[1:]))
	}

	os.Exit(m.Run())
}

// fakeVBoxManage stands in for VBoxManage. It appends its arguments, joined
// with spaces, as a line to the file named by FAKE_VBM_LOG, and then answers
// according to the first argument:
//
//	--version  prints the contents of FAKE_VBM_VERSION, or 7.1.4r165100
//	import     exits with an error, so that a test can see how it was called
//	           without anything else being done
//
// Anything else makes it exit with an error that says so.
func fakeVBoxManage(args []string) int {
	if logpath := os.Getenv("FAKE_VBM_LOG"); logpath != "" {
		f, err := os.OpenFile(logpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
		if err == nil {
			fmt.Fprintln(f, strings.Join(args, " "))
			f.Close()
		}
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "fake VBoxManage: no arguments")
		return 2
	}

	switch args[0] {
	case "--version":
		version := os.Getenv("FAKE_VBM_VERSION")
		if version == "" {
			version = "7.1.4r165100"
		}
		fmt.Println(version)
		return 0

	case "import":
		fmt.Fprintln(os.Stderr, "fake VBoxManage: import failed")
		return 1
	}

	fmt.Fprintln(os.Stderr, "fake VBoxManage: unexpected arguments:", args)
	return 2
}

// testStorage points the kutti workspace at a new temporary directory for the
// duration of a test, and makes the driver use the fake VBoxManage. It
// returns the workspace directory. The workspace and the driver are global,
// so tests that use this must not run in parallel.
func testStorage(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := workspace.Set(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(workspace.Reset)

	kuttilog.SetLogger(quietlogger{})
	t.Cleanup(kuttilog.ResetLogger)

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	saved := findvboxmanage
	findvboxmanage = func() (string, error) { return exe, nil }
	t.Cleanup(func() { findvboxmanage = saved })

	t.Setenv(fakeVBoxManageEnv, "1")
	t.Setenv("FAKE_VBM_LOG", filepath.Join(t.TempDir(), "calls.log"))
	t.Setenv("FAKE_VBM_VERSION", "")

	return root
}

// noVBoxManage makes the driver find no VBoxManage.
func noVBoxManage(t *testing.T) {
	t.Helper()

	saved := findvboxmanage
	findvboxmanage = func() (string, error) { return "", fmt.Errorf("VBoxManage not found") }
	t.Cleanup(func() { findvboxmanage = saved })
}

// vbmCalls returns the calls made to the fake VBoxManage so far, one string
// for each, of the arguments joined with spaces.
func vbmCalls(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(os.Getenv("FAKE_VBM_LOG"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// The locations of things, written out in full on purpose, so that a change
// to the layout is noticed.

func cacheDirPath(root string) string {
	return filepath.Join(root, "kutti-cache", "driver-vbox")
}

func machinesDirPath(root string) string {
	return filepath.Join(root, "kutti-cache", "driver-vbox-machines")
}

func listFilePath(root string) string {
	return filepath.Join(root, "kutti-config", "driver-vbox-images.json")
}

// cachedImagePath is where the cached image for a version of Kubernetes must
// be.
func cachedImagePath(root string, version string) string {
	return filepath.Join(cacheDirPath(root), "kutti-"+version+".ova")
}

// imageFiles returns the sorted names of the entries of the cache directory.
// A cache directory that does not exist yet holds nothing.
func imageFiles(t *testing.T, root string) []string {
	t.Helper()

	entries, err := os.ReadDir(cacheDirPath(root))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	return names
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o666); err != nil {
		t.Fatal(err)
	}
}
