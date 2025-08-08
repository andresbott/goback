package mysqldump

import (
	"context"
	"strings"
	"testing"

	"github.com/AndresBott/goback/lib/ssh"
	"github.com/google/go-cmp/cmp"
)

func TestWriteFromRemote(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

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

	_ = cl.Connect()

	cfg := RemoteCfg{
		BinPath: "/usr/local/bin/mysqldump",
		User:    "user",
		Pw:      "pass",
		DbName:  "testDbName",
	}

	// Create a buffer to capture the output
	var output strings.Builder

	err = WriteFromRemote(cl, cfg, &output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectContent := "mysqldump mock binary, params: -u user -ppass --add-drop-database --databases testDbName\n"
	gotContent := output.String()

	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestGetRemoteBinPath(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

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

	_ = cl.Connect()

	// Test that we can find the mysqldump binary
	binPath, err := GetRemoteBinPath(cl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mock mysqldump should be at /usr/local/bin/mysqldump
	expectedPath := "/usr/local/bin/mysqldump"
	if binPath != expectedPath {
		t.Errorf("expected binPath to be %s, got %s", expectedPath, binPath)
	}
}

func TestWriteFromRemoteWithAutoBinPath(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

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

	_ = cl.Connect()

	// Test with empty BinPath to trigger auto-discovery
	cfg := RemoteCfg{
		BinPath: "", // This should trigger GetRemoteBinPath
		User:    "user",
		Pw:      "pass",
		DbName:  "testDbName",
	}

	// Create a buffer to capture the output
	var output strings.Builder

	err = WriteFromRemote(cl, cfg, &output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectContent := "mysqldump mock binary, params: -u user -ppass --add-drop-database --databases testDbName\n"
	gotContent := output.String()

	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}
