package goback

import (
	"context"
	"github.com/AndresBott/goback/app/logger"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/sftp"
	"os"
	"path/filepath"
	"testing"
)

func TestPullProfiles(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

	setup := func(t *testing.T) string {
		tmpdir := t.TempDir()
		files := []string{
			"a.zip",
			"bla",
			"blib_2011_02_05-17:04:05_backup.zip",
		}

		for _, f := range files {
			content := []byte("hello\ngo\n")
			e := os.WriteFile(filepath.Join(tmpdir, f), content, 0600)
			if e != nil {
				t.Fatal(e)
			}
		}
		return tmpdir
	}

	t.Run("pull backups from remote", func(t *testing.T) {
		tmpdir := setup(t)
		sshClient, err := ssh.New(ssh.Cfg{
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

		err = sshClient.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer func() {
			_ = sshClient.Disconnect()
		}()

		sftpClient, err := sftp.NewClient(sshClient.Connection())
		if err != nil {
			t.Fatalf("unable to create sftp client %v", err)
		}
		defer func() {
			_ = sftpClient.Close()
		}()

		err = syncRemoteBackups(sftpClient, "/backupDestination", "blib", tmpdir, logger.SilentLogger())
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		files, _ := os.ReadDir(tmpdir)
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
			got, _ := findDifferentProfiles(tc.remote, tc.local, tc.prfName)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
