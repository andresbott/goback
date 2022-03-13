package goback

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	_ "github.com/davecgh/go-spew/spew"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
)

func TestZipName(t *testing.T) {

	// i know this test does not prove anything, but i needed something to actually call the function
	got := getZipName("bla")

	dt := time.Now()
	dateStr := "2006_02_01-15:04:05"
	want := "bla_" + dt.Format(dateStr) + "_backup.zip"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func listFilesInZip(in string) ([]string, error) {
	read, err := zip.OpenReader(in)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %s", err)
	}
	defer read.Close()

	files := []string{}
	for _, file := range read.File {
		files = append(files, file.Name)
	}
	return files, nil
}

func readFileInZio(zipFile string, file string) (string, error) {

	zf, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}
	defer zf.Close()

	buf := bytes.Buffer{}
	for _, f := range zf.File {
		if f.Name != file {
			continue
		}

		// Found it, print its content to terminal:
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		_, err = io.Copy(&buf, rc)
		if err != nil {
			return "", err
		}
		rc.Close()
		break
	}

	return buf.String(), nil
}

func getGlob(in string) glob.Glob {
	return glob.MustCompile(in)
}
