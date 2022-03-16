package goback

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPullProfiles(t *testing.T) {
	skipIfCi(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sshServer.Terminate(ctx)

	setup := func(t *testing.T) (func(t *testing.T), string) {

		// make the tmp dir
		dir, err := os.MkdirTemp("", strings.ReplaceAll(t.Name(), "/", "_"))
		if err != nil {
			log.Fatal(err)
		}

		files := []string{
			"a.zip",
			"bla",
			"blib_2011_02_05-17:04:05_backup.zip",
		}

		for _, f := range files {
			d1 := []byte("hello\ngo\n")
			os.WriteFile(filepath.Join(dir, f), d1, 0644)
		}

		return func(t *testing.T) {
			os.RemoveAll(dir)
		}, dir
	}

	t.Run("pull backups from remote", func(t *testing.T) {

		destroy, dir := setup(t)
		defer destroy(t)

		cl, err := ssh.New(ssh.Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          ssh.Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer cl.Disconnect()

		err = syncBackups(cl, "/backupDestination", dir, "blib")
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		files, _ := os.ReadDir(dir)
		got := []string{}
		for _, f := range files {
			if !f.IsDir() {
				got = append(got, f.Name())
			}
		}

		want := []string{
			"a.zip",
			"bla",
			"blib_2010_02_05-17:04:05_backup.zip",
			"blib_2011_02_05-17:04:05_backup.zip",
			"blib_2012_02_05-17:04:05_backup.zip",
		}

		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("output mismatch (-got +want):\n%s", diff)
		}

	})

}

func TestFindDifferentProfiles(t *testing.T) {
	tcs := []struct {
		tcName    string
		remote    []string
		local     []string
		prfName   string
		want      []string
		expectErr string
	}{
		{
			tcName: "basic",
			remote: []string{
				"blib_2006_02_05-17:04:05_backup.zip",
				"ble_2008_02_05-17:04:05_backup.zip",
				"blib_2011_02_05-17:04:05_backup.zip",
				"blib_2012_02_05-17:04:05_backup.zip",
				"blib_2009_02_05-17:04:05_backup.zip",
				"blib_2007_02_05-17:04:05_backup.zip",
				"bli_2008_02_05-17:04:05_backup.zip",
			},
			local: []string{
				"blib_2004_02_05-17:04:05_backup.zip",
				"blib_2005_02_05-17:04:05_backup.zip",
				"blib_2007_02_05-17:04:05_backup.zip",
				"blib_2009_02_05-17:04:05_backup.zip",
			},
			prfName: "blib",
			want: []string{
				"blib_2006_02_05-17:04:05_backup.zip",
				"blib_2011_02_05-17:04:05_backup.zip",
				"blib_2012_02_05-17:04:05_backup.zip",
			},
		},

		{
			tcName: "empty local",
			remote: []string{
				"blib_2006_02_05-17:04:05_backup.zip",
				"ble_2008_02_05-17:04:05_backup.zip",
				"blib_2011_02_05-17:04:05_backup.zip",
			},
			local:   []string{},
			prfName: "blib",
			want: []string{
				"blib_2006_02_05-17:04:05_backup.zip",
				"blib_2011_02_05-17:04:05_backup.zip",
			},
		},

		{
			tcName: "empty remote",
			remote: []string{
				"ble_2008_02_05-17:04:05_backup.zip",
			},
			local:   []string{},
			prfName: "blib",
			want:    []string{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.tcName, func(t *testing.T) {

			got := findDifferentProfiles(tc.remote, tc.local, tc.prfName)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
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
			Context:    "./sampledata",
			Dockerfile: "Dockerfile",
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

func TestSFTP(t *testing.T) {

	tcs := []struct {
		name          string
		dirs          []profile.BackupDir
		mysql         []profile.MysqlBackup
		expectedFiles []string
		expectedErr   string
	}{
		{
			name: "expect profile with single directory",
			dirs: []profile.BackupDir{
				{
					Root:    "/data/dir1",
					Exclude: nil,
				},
			},
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile.log",
				"dir1/subdir1/subfile1.txt",
			},
		},

		{
			name: "expect profile with multiple directory and excluded glob",
			dirs: []profile.BackupDir{
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
			expectedFiles: []string{
				"dir1/file.json",
				"dir1/subdir1/subfile1.txt",
				"dir2/file.yaml",
			},
		},

		{
			name: "expect database to be backedup",
			mysql: []profile.MysqlBackup{
				{
					DbName: "mydb",
					User:   "user",
					Pw:     "pw",
				},
			},
			expectedFiles: []string{
				"_mysqldump/mydb.dump.slq",
			},
		},
	}

	skipIfCi(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sshServer.Terminate(ctx)

	setup := func(t *testing.T) (func(t *testing.T), string) {

		// make the tmp dir
		dir, err := os.MkdirTemp("", strings.ReplaceAll(t.Name(), "/", "_"))
		if err != nil {
			log.Fatal(err)
		}

		return func(t *testing.T) {
			os.RemoveAll(dir)
		}, dir
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			destroy, dir := setup(t)
			defer destroy(t)
			zipFile := filepath.Join(dir, "test.zip")

			cl, err := ssh.New(ssh.Cfg{
				Host:          sshServer.host,
				Port:          sshServer.port,
				Auth:          ssh.Password,
				User:          "pwuser",
				Password:      "1234",
				IgnoreHostKey: true,
			})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			err = cl.Connect()
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			defer cl.Disconnect()

			err = backupSftp(cl, tc.dirs, tc.mysql, zipFile)
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
