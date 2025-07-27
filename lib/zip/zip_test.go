package zip

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestNewZipWriter(t *testing.T) {

	t.Run("ensure no error is returned", func(t *testing.T) {
		dir := t.TempDir()
		_, err := New(dir + "/bla.zip")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ensure error about zip extension", func(t *testing.T) {
		dir := t.TempDir()
		_, err := New(dir + "bla")

		expect := "destination does not end in .zip"
		if err.Error() != expect {
			t.Fatalf("unexpected error: %v, expecting %s", err, expect)
		}
	})
}

func TestAddFileToZip(t *testing.T) {

	setupTest := func(t *testing.T) (*Handler, string) {
		dir := t.TempDir()
		zipFile := dir + "/bla.zip"

		zh, err := New(zipFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return zh, zipFile
	}

	t.Run("correctly add 2 files", func(t *testing.T) {
		zh, zipFile := setupTest(t)

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
