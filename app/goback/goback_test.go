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

func TestBackupProfile_local(t *testing.T) {

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
				"dir2/.hidden",
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
				"_mysqldump/mydb.dump.sql",
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

		{
			name: "expect symbolic link to be preserved",
			profile: profile.Profile{
				Name: "bli",
				Dirs: []profile.BackupDir{
					{
						Root: "sampledata/files/",
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
			zipFile := filepath.Join(tmpDir, "out", "test.zip")
			tc.profile.Destination = filepath.Join(tmpDir, "out")

			err := BackupProfile(tc.profile, logger.SilentLogger(), "test.zip")

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

func TestBackupProfile_remote(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	tcs := []struct {
		name          string
		profile       profile.Profile
		dirs          []profile.BackupDir
		mysql         []profile.MysqlBackup
		expectedFiles []string
		expectedErr   string
	}{
		{
			name: "expect profile with single directory",
			profile: profile.Profile{
				Name:     "bli",
				IsRemote: true,
				Remote:   profile.RemoteCfg{},
				Dirs: []profile.BackupDir{
					{
						Root:    "/data/dir1",
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
				Name:     "bli",
				IsRemote: true,
				Remote:   profile.RemoteCfg{},
				Dirs: []profile.BackupDir{
					{
						Root: "/data/dir1",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
					{
						Root:    "/data/dir2",
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
				Name:     "bli",
				IsRemote: true,
				Remote:   profile.RemoteCfg{},
				Mysql: []profile.MysqlBackup{
					{
						DbName: "mydb",
						User:   "user",
						Pw:     "pw",
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
	defer sshServer.Terminate(ctx)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			zipFile := filepath.Join(tmpDir, "out", "test.zip")

			tc.profile.Destination = filepath.Join(tmpDir, "out")
			tc.profile.Remote = profile.RemoteCfg{
				Host:     sshServer.host,
				Port:     strconv.Itoa(sshServer.port),
				AuthType: profile.RemotePassword,
				User:     "pwuser",
				Password: "1234",
			}
			ignoreHostKey = true // ignore for tests only

			err = BackupProfile(tc.profile, logger.SilentLogger(), "test.zip")
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
	defer read.Close()

	files := []string{}
	for _, file := range read.File {
		files = append(files, file.Name)
	}
	return files, nil
}

func getGlob(in string) glob.Glob {
	return glob.MustCompile(in)
}
