package goback

import (
	"github.com/AndresBott/goback/app/logger"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestExpurgeDir(t *testing.T) {
	createFile := func(path string) {
		emptyFile, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		emptyFile.Close()
	}

	setup := func() (string, func()) {
		dir, err := os.MkdirTemp("", t.Name())
		if err != nil {
			log.Fatal(err)
		}
		// destructor function
		destructor := func() {
			os.RemoveAll(dir)
		}
		return dir, destructor
	}

	dir, dest := setup()
	defer dest()

	createFile(filepath.Join(dir, "blib_2007_02_05-17:04:05_backup.zip"))
	createFile(filepath.Join(dir, "blib_2006_02_05-17:04:05_backup.zip"))
	createFile(filepath.Join(dir, "name_2008_02_05-17:04:05_backup.zip"))

	err := ExpurgeDir(dir, 1, "blib", logger.SilentLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, err := filepath.Glob(dir + "/*.zip")
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, file := range files {
		got = append(got, filepath.Base(file))
	}

	want := []string{
		"blib_2007_02_05-17:04:05_backup.zip",
		"name_2008_02_05-17:04:05_backup.zip",
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestFindToDelete(t *testing.T) {

	tcs := []struct {
		name        string
		profileName string
		keepOld     int
		in          []string
		expect      []string
	}{
		{
			name:        "directory contains other profiles and zero matching, none to delete",
			profileName: "blib",
			keepOld:     2,
			in: []string{
				"proile_name_2006_02_01-15:04:05_backup.zip",
				"ble_2006_02_01-16:04:05_backup.zip",
				"bli_2006_02_01-17:04:05_backup.zip",
				"name_2006_02_01-18:04:05_backup.zip",
			},
			expect: []string{},
		},
		{
			name:        "directory contains other profiles, one matching, keep four, none to delete",
			profileName: "blip",
			keepOld:     4,
			in: []string{
				"name_2006_02_01-15:04:05_backup.zip",
				"blip_2006_02_01-16:04:05_backup.zip",
			},
			expect: []string{},
		},
		{
			name:        "directory contains other profiles, and four matching, keep two, two to delete",
			profileName: "blib",
			keepOld:     2,
			in: []string{
				"name_2006_02_01-15:04:05_backup.zip",
				"ble_2006_02_02-16:04:05_backup.zip",
				"blib_2006_02_05-17:04:05_backup.zip",
				"blib_2007_02_03-17:04:05_backup.zip",
				"name_2006_02_04-18:04:05_backup.zip",
				"name_2006_02_06-18:04:05_backup.zip",
				"blib_2006_02_10-17:04:05_backup.zip",
				"name_2006_02_07-18:04:05_backup.zip",
				"name_2006_02_09-18:04:05_backup.zip",
				"blib_2006_02_08-17:04:05_backup.zip",
			},
			expect: []string{
				"blib_2006_02_05-17:04:05_backup.zip",
				"blib_2006_02_08-17:04:05_backup.zip",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			got, err := findToDelete(tc.in, tc.profileName, tc.keepOld)

			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if diff := cmp.Diff(tc.expect, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}

		})
	}
}

func TestExtractTome(t *testing.T) {

	genTime := func(in string) time.Time {
		t, _ := time.Parse(dateStr, in)
		return t
	}

	tcs := []struct {
		name   string
		in     string
		expect time.Time
	}{
		{
			name:   "profile name with underscore",
			in:     "profile_name_2020_11_05-05:02:32_backup.zip",
			expect: genTime("2020_11_05-05:02:32"),
		},
		{
			name:   "simple profile name",
			in:     "name_2021_11_05-05:02:32_backup.zip",
			expect: genTime("2021_11_05-05:02:32"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			got := extractTime(tc.in)

			if diff := cmp.Diff(tc.expect, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}

		})
	}

}
