package drivervbox_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	drivervbox "github.com/kuttiproject/driver-vbox"
	"github.com/kuttiproject/drivercore/drivercoretest"
	"github.com/kuttiproject/workspace"
)

// The version and checksum of the driver-vbox image
// to use for the test.
const (
	TESTK8SVERSION  = "1.32"
	TESTK8SCHECKSUM = ""
)

func TestDriverVBox(t *testing.T) {
	// kuttilog.Setloglevel(kuttilog.Debug)

	// Set up dummy web server for updating image list
	// and downloading image
	_, err := os.Stat(fmt.Sprintf("out/testserver/kutti-%v.ova", TESTK8SVERSION))
	if err != nil {
		t.Fatalf(
			"Please download the version %v kutti image, and place it in the path out/testserver/kutti-%v.ova. Also update the checksum in drivervbox_test.go.",
			TESTK8SVERSION,
			TESTK8SVERSION,
		)
	}

	if TESTK8SCHECKSUM == "" {
		t.Fatalf("Please update the checksum in drivervbox_test.go.")
	}

	serverMux := http.NewServeMux()
	server := http.Server{Addr: "localhost:8181", Handler: serverMux}
	defer server.Shutdown(context.Background())

	serverMux.HandleFunc(
		"/images.json",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(
				w,
				`{"%v":{"ImageK8sVersion":"%v","ImageChecksum":"%v","ImageStatus":"NotDownloaded", "ImageSourceURL":"http://localhost:8181/kutti-%v.ova"}}`,
				TESTK8SVERSION,
				TESTK8SVERSION,
				TESTK8SCHECKSUM,
				TESTK8SVERSION,
			)
		},
	)

	serverMux.HandleFunc(
		fmt.Sprintf("/kutti-%v.ova", TESTK8SVERSION),
		func(rw http.ResponseWriter, r *http.Request) {
			http.ServeFile(
				rw,
				r,
				fmt.Sprintf("out/testserver/kutti-%v.ova", TESTK8SVERSION),
			)
		},
	)

	go func() {
		t.Log("Server starting...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			t.Logf("ERROR:%v", err)
		}
		t.Log("Server stopped.")
	}()

	t.Log("Waiting 5 seconds for dummy server to start.")

	<-time.After(5 * time.Second)

	err = workspace.Set("out/")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	drivervbox.ImagesSourceURL = "http://localhost:8181/images.json"

	drivercoretest.TestDriver(t, "vbox", TESTK8SVERSION)
}
