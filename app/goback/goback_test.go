package goback

import (
	"archive/zip"
	"context"
	"fmt"
	"github.com/AndresBott/goback/app/logger"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	_ "github.com/davecgh/go-spew/spew"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
)

// TestZipName ensure zip names are consistent
func TestZipName(t *testing.T) {
	got := getZipName("bla")
	dt := time.Now()
	dateStr := "2006_02_01-15:04:05"
	want := "bla_" + dt.Format(dateStr) + "_backup.zip"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestBackupLocal(t *testing.T) {

	tcs := []struct {
		name          string
		profile       profile.Profile
		expectedFiles []string
		expectedErr   string
	}{
		{
			name: "profile with single directory",
			profile: profile.Profile{
				Name: "bla",
				Dirs: []profile.BackupPath{
					{
						Path:    "sampledata/files/dir1",
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
			name: "profile with multiple directory",
			profile: profile.Profile{
				Name: "bla",
				Dirs: []profile.BackupPath{
					{
						Path:    "sampledata/files/dir1",
						Exclude: nil,
					},
					{
						Path:    "sampledata/files/dir2",
						Exclude: nil,
					},
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
				"dir2/.hidden",
				"dir2/file.yaml",
			},
		},

		{
			name: "profile with files and database",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupPath{
					{
						Path:    "sampledata/files/dir1",
						Exclude: nil,
					},
				},
				Dbs: []profile.BackupDb{
					{
						Type:     profile.DbMysql,
						Name:     "mydb",
						User:     "user",
						Password: "pw",
					},
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
				"_mysqldump/mydb.dump.sql",
			},
		},

		{
			name: "expect err with generated zip file to be removed",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupPath{
					{
						Path:    "sampledata/files/dir1",
						Exclude: nil,
					},
				},
				Dbs: []profile.BackupDb{
					{
						Type:     profile.DbMysql,
						Name:     "mydb",
						User:     "fail", // the crafted mysqldump binary fails if the username is fail
						Password: "pw",
					},
				},
			},
			expectedErr: "error running mysqldump: exit status 1",
		},

		{
			name: "expect symbolic link to be preserved",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupPath{
					{
						Path: "sampledata/files/",
						Exclude: []glob.Glob{
							getGlob("sampledata/files/dir1/*"),
						},
					},
				},
			},
			expectedFiles: []string{
				"files/dir2/.hidden",
				"files/dir2/file.yaml",
				"files/notRoot/link",
			},
		},
	}

	setup := func(t *testing.T) string {
		// overwrite PATH to contain the dummy mysqldump
		pathEnv := os.Getenv("PATH")
		binPath, _ := filepath.Abs("./sampledata")
		t.Setenv("PATH", pathEnv+":"+binPath)
		tmpdir := t.TempDir()
		return tmpdir
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			tmpDir := setup(t)
			zipFile := filepath.Join(tmpDir, "test.zip")
			tc.profile.Destination.Path = tmpDir

			err := backupLocal(tc.profile, zipFile, logger.SilentLogger())

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

				files, _ := os.ReadDir(filepath.Join(tmpDir, "out"))
				if len(files) != 0 {
					t.Errorf("expecting output file to be deleted, but found %d files in the directory", len(files))
				}

			}
		})
	}
}

type sshContainer struct {
	testcontainers.Container
	host string
	port int
}

func setupContainer(ctx context.Context) (*sshContainer, error) {

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "./sampledata",
			Dockerfile:    "Dockerfile",
			PrintBuildLog: false, // set to true to troubleshoot docker build issues
		},
		ExposedPorts: []string{"22/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server listening on 0.0.0.0 port 22"),
		),
		Env: map[string]string{
			"PW_USER": "kTZ8GVSkARoNg", // user: pwuser pw: 1234
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, err
	}

	//losg,_ := container.Logs(ctx)
	//lines,_ := io.ReadAll(losg)
	//fmt.Println(string(lines))

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "22")
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(mappedPort.Port())

	sshCont := &sshContainer{
		Container: container,
		host:      ip,
		port:      port,
	}

	return sshCont, nil
}

func TestBackupRemote(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	tcs := []struct {
		name          string
		profile       profile.Profile
		dirs          []profile.BackupPath
		mysql         []profile.BackupDb
		expectedFiles []string
		expectedErr   string
	}{
		{
			name: "expect profile with single directory",
			profile: profile.Profile{
				Name: "bli",
				Ssh:  profile.Ssh{},
				Dirs: []profile.BackupPath{
					{
						Path:    "/data/dir1",
						Exclude: nil,
					},
				},
			},
			expectedFiles: []string{
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
				"dir1/file.json",
			},
		},

		{
			name: "expect profile with multiple directory and excluded glob",
			profile: profile.Profile{
				Name: "bli",

				Ssh: profile.Ssh{},
				Dirs: []profile.BackupPath{
					{
						Path: "/data/dir1",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
					{
						Path:    "/data/dir2",
						Exclude: nil,
					},
				},
			},
			expectedFiles: []string{
				"dir1/subdir1/subfile1.txt",
				"dir1/file.json",
				"dir2/file.yaml",
				"dir2/.hidden",
			},
		},

		{
			name: "expect database to be backedup",
			profile: profile.Profile{
				Name: "bli",

				Ssh: profile.Ssh{},
				Dbs: []profile.BackupDb{
					{
						Type:     profile.DbMysql,
						Name:     "mydb",
						User:     "user",
						Password: "pw",
					},
				},
			},
			expectedFiles: []string{
				"_mysqldump/mydb.dump.sql",
			},
		},
	}

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			zipFile := filepath.Join(tmpDir, "test.zip")

			tc.profile.Destination.Path = tmpDir

			tc.profile.Ssh = profile.Ssh{
				Type:     profile.ConnTypePasswd,
				Host:     sshServer.host,
				Port:     sshServer.port,
				User:     "pwuser",
				Password: "1234",
			}
			ignoreHostKey = true // ignore for tests only

			err = backupRemote(tc.profile, zipFile, logger.SilentLogger())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := listFilesInZip(zipFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := tc.expectedFiles
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}

		})
	}
}

func skipInCI(t *testing.T) {
	envSet := os.Getenv("RUN_TESTCONTAINERS")
	if envSet == "" {
		t.Skip("skipping because env \"RUN_TESTCONTAINERS\" is not set to true")
	}
}

func listFilesInZip(in string) ([]string, error) {
	read, err := zip.OpenReader(in)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %s", err)
	}
	defer func() {
		_ = read.Close()
	}()

	files := []string{}
	for _, file := range read.File {
		files = append(files, file.Name)
	}
	return files, nil
}

func getGlob(in string) glob.Glob {
	return glob.MustCompile(in)
}
