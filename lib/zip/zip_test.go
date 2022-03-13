package zip

import (
	"github.com/google/go-cmp/cmp"
	"log"
	"os"
	"testing"
)

func TestNewZipWriter(t *testing.T) {

	t.Run("ensure no error is returned", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "TestNewTestWriter")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir)

		_, err = New(dir + "/bla.zip")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ensure error about zip extension", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "TestNewTestWriter")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir)

		_, err = New(dir + "bla")

		expect := "destination does not end in .zip"
		if err.Error() != expect {
			t.Fatalf("unexpected error: %v, expecting %s", err, expect)
		}
	})
}

func TestAddFileToZip(t *testing.T) {

	setupTest := func(t *testing.T) (func(t *testing.T), *Handler, string) {
		dir, err := os.MkdirTemp("", "TestAddFileToZip")
		if err != nil {
			log.Fatal(err)
		}
		zipFile := dir + "/bla.zip"

		zh, err := New(zipFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		return func(t *testing.T) {
			os.RemoveAll(dir)
		}, zh, zipFile
	}

	t.Run("correctly add 2 files", func(t *testing.T) {
		destroy, zh, zipFile := setupTest(t)
		defer destroy(t)

		err := zh.AddFile("sampledata/files/dir1/subdir1/subfile.log", "sampledata/files/dir1/subdir1/subfile.log")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = zh.AddFile("sampledata/files/dir1/file.json", "sampledata/files/dir1/file.json")
		zh.Close()

		got, err := ListFiles(zipFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{
			"sampledata/files/dir1/subdir1/subfile.log",
			"sampledata/files/dir1/file.json",
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})
}
