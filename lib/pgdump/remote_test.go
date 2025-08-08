package pgdump

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
		BinPath: "/usr/local/bin/pg_dump",
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

	expectContent := "pg_dump mock binary, params: -U user -W --clean --if-exists --create --verbose testDbName\n"
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

	// Test that we can find the pg_dump binary
	binPath, err := GetRemoteBinPath(cl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mock pg_dump should be at /usr/local/bin/pg_dump
	expectedPath := "/usr/local/bin/pg_dump"
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

	expectContent := "pg_dump mock binary, params: -U user -W --clean --if-exists --create --verbose testDbName\n"
	gotContent := output.String()

	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestNewRemote(t *testing.T) {
	cfg := RemoteCfg{
		BinPath: "/usr/bin/pg_dump",
		User:    "testuser",
		Pw:      "testpass",
		DbName:  "testdb",
	}

	h, err := NewRemote(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "/usr/bin/pg_dump" {
		t.Errorf("expected bin path /usr/bin/pg_dump, got: %s", h.binPath)
	}

	if h.user != "testuser" {
		t.Errorf("expected user testuser, got: %s", h.user)
	}

	if h.pw != "testpass" {
		t.Errorf("expected pw testpass, got: %s", h.pw)
	}

	if h.dbName != "testdb" {
		t.Errorf("expected db name testdb, got: %s", h.dbName)
	}
}

func TestNewRemoteDefaultBinPath(t *testing.T) {
	cfg := RemoteCfg{
		User:   "testuser",
		Pw:     "testpass",
		DbName: "testdb",
	}

	h, err := NewRemote(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "pg_dump" {
		t.Errorf("expected default bin path pg_dump, got: %s", h.binPath)
	}
}

func TestRemoteCmd(t *testing.T) {
	h := RemoteHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "testuser",
		pw:      "testpass",
		dbName:  "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump -U testuser -W --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

func TestRemoteCmdNoUser(t *testing.T) {
	h := RemoteHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "",
		pw:      "",
		dbName:  "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}
