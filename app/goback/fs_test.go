package goback

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
)

// TestBackupProfile is a full end to end test of the backup functionality
// this includes file system and database dump
func TestBackupLocalFs(t *testing.T) {

	tcs := []struct {
		name          string
		profile       profile.Profile
		expectedFiles []string
		expectedErr   string
	}{
		{
			name: "expect profile with single directory",
			profile: profile.Profile{
				Name: "bla",
				Dirs: []profile.BackupDir{
					{
						Root: "sampledata/files/dir1",

						Exclude: nil,
					},
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
			},
		},

		{
			name: "expect profile with multiple directory",
			profile: profile.Profile{
				Name: "bla",
				Dirs: []profile.BackupDir{
					{
						Root:    "sampledata/files/dir1",
						Exclude: nil,
					},
					{
						Root:    "sampledata/files/dir2",
						Exclude: nil,
					},
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
				"dir2/file.yaml",
			},
		},

		{
			name: "expect profile with files and database",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupDir{
					{
						Root:    "sampledata/files/dir1",
						Exclude: nil,
					},
				},
				Mysql: []profile.MysqlBackup{
					{
						DbName: "mydb",
						User:   "user",
						Pw:     "pw",
					},
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
				"_mysqldump/mydb.dump.slq",
			},
		},

		{
			name: "expect generated zip file to be removed",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupDir{
					{
						Root:    "sampledata/files/dir1",
						Exclude: nil,
					},
				},
				Mysql: []profile.MysqlBackup{
					{
						DbName: "mydb",
						User:   "fail", // the crafted mysqldump binary fails if the username is fail
						Pw:     "pw",
					},
				},
			},
			expectedErr: "error running mysqldump: exit status 1",
		},
	}

	setup := func(t *testing.T) (func(t *testing.T), string) {

		// overwrite PATH to contain the dummy mysqldump
		pathEnv := os.Getenv("PATH")
		binPath, _ := filepath.Abs("./sampledata")
		os.Setenv("PATH", pathEnv+":"+binPath)

		// make the tmp dir
		dir, err := os.MkdirTemp("", strings.ReplaceAll(t.Name(), "/", "_"))
		if err != nil {
			log.Fatal(err)
		}

		return func(t *testing.T) {
			os.Setenv("PATH", pathEnv)
			os.RemoveAll(dir)
		}, dir
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			destroy, dir := setup(t)
			defer destroy(t)
			zipFile := filepath.Join(dir, "test.zip")

			err := backupLocalFs(tc.profile.Dirs, tc.profile.Mysql, zipFile)

			if tc.expectedErr == "" {

				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				got, err := listFilesInZip(zipFile)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if diff := cmp.Diff(tc.expectedFiles, got); diff != "" {
					t.Errorf("output mismatch (-want +got):\n%s", diff)
				}

			} else {
				if err == nil {
					t.Fatalf("expecing error: \"%s\" but none returned", tc.expectedErr)
				}
				if tc.expectedErr != err.Error() {
					t.Fatalf("expecting message error: \"%s\", but got: \"%s\"", tc.expectedErr, err.Error())
				}

				files, _ := ioutil.ReadDir(dir)
				if len(files) != 0 {
					t.Errorf("expecting output file to be deleted, but found %d files in the directory", len(files))
				}

			}
		})
	}
}

type fileAppender struct {
	files []string
}

func (a *fileAppender) AddFile(origin string, dest string) error {
	a.files = append(a.files, dest)
	return nil
}

func TestDumpFileSystem(t *testing.T) {

	tcs := []struct {
		name    string
		profile profile.BackupDir
		want    []string
	}{
		{
			name: "expect all files processed correctly",
			profile: profile.BackupDir{
				Root:    "sampledata/files",
				Exclude: nil,
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir1/subdir1/subfile.log",
				"files/dir1/subdir1/subfile1.txt",
				"files/dir2/file.yaml",
			},
		},

		{
			name: "expect files to be filtered out, multiple filters",
			profile: profile.BackupDir{
				Root: "sampledata/files",
				Exclude: []glob.Glob{
					getGlob("*.log"),
					getGlob("*.txt"),
				},
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir2/file.yaml",
			},
		},

		{
			name: "expect full directory to be excluded",
			profile: profile.BackupDir{
				Root: "sampledata/files",
				Exclude: []glob.Glob{
					getGlob("sampledata/files/dir1/subdir1/*"),
				},
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir2/file.yaml",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			fa := fileAppender{}
			err := dumpFileSystem(tc.profile, &fa)
			got := fa.files

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}

		})
	}
}
