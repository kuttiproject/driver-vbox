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

func TestDriverVBox(t *testing.T) {
	// kuttilog.Setloglevel(kuttilog.Debug)

	// Set up dummy web server for updating image list
	// and downloading image
	_, err := os.Stat("out/testserver/kutti-1.18.ova")
	if err != nil {
		t.Fatal(
			"Please download the version 1.18 kutti image, and place it in the path out/testserver/kutti-1.18.ova",
		)
	}

	serverMux := http.NewServeMux()
	server := http.Server{Addr: "localhost:8181", Handler: serverMux}
	defer server.Shutdown(context.Background())

	serverMux.HandleFunc(
		"/images.json",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"1.18":{"ImageK8sVersion":"1.18","ImageChecksum":"a053e6910c55e19bbd2093b0129f25aa69ceee9b0e0a52505dfd9d8b3eb24090","ImageStatus":"NotDownloaded", "ImageSourceURL":"http://localhost:8181/kutti-1.18.ova"}}`)
		},
	)

	serverMux.HandleFunc(
		"/kutti-1.18.ova",
		func(rw http.ResponseWriter, r *http.Request) {
			http.ServeFile(
				rw,
				r,
				"out/testserver/kutti-1.18.ova",
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

	err = workspace.Set("/home/raj/projects/kuttiproject/driver-vbox/out")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	drivervbox.ImagesSourceURL = "http://localhost:8181/images.json"

	drivercoretest.TestDriver(t, "vbox", "1.18")
}
