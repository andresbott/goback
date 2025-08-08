package mysqldump

import (
	"context"
	"log"
	"testing"

	"github.com/AndresBott/goback/lib/ssh"
	zipHandler "github.com/AndresBott/goback/lib/zip"
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

	setup := func(t *testing.T) (string, *zipHandler.Handler) {
		dir := t.TempDir()
		zipFile := dir + "/test_zip.zip"

		zh, err := zipHandler.New(zipFile)
		if err != nil {
			log.Fatal(err)
		}
		return zipFile, zh
	}
	zipFile, zh := setup(t)

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

	err = WriteFromRemote(cl, cfg, zh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	zh.Close()

	got, err := listFilesInZip(zipFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expect := []string{
		"_mysqldump/testDbName.dump.sql",
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}

	expectContent := "mysqldump mock binary, params: -u user -ppass --add-drop-database --databases testDbName\n"
	gotContent, err := readFileInZip(zipFile, "_mysqldump/testDbName.dump.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	setup := func(t *testing.T) (string, *zipHandler.Handler) {
		dir := t.TempDir()
		zipFile := dir + "/test_zip.zip"

		zh, err := zipHandler.New(zipFile)
		if err != nil {
			log.Fatal(err)
		}
		return zipFile, zh
	}
	zipFile, zh := setup(t)

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

	err = WriteFromRemote(cl, cfg, zh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	zh.Close()

	got, err := listFilesInZip(zipFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expect := []string{
		"_mysqldump/testDbName.dump.sql",
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}

	expectContent := "mysqldump mock binary, params: -u user -ppass --add-drop-database --databases testDbName\n"
	gotContent, err := readFileInZip(zipFile, "_mysqldump/testDbName.dump.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}
