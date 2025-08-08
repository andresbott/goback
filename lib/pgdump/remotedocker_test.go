package pgdump

import (
	"context"
	"strings"
	"testing"

	"github.com/AndresBott/goback/lib/ssh"
)

func TestGetSshDockerBinPathRequiresDocker(t *testing.T) {
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

	// Test that it fails when Docker is not available
	// Since the test container doesn't have Docker, this should fail
	_, err = GetSshDockerBinPath(cl, "postgres-container")
	if err == nil {
		t.Fatalf("expected error when Docker is not available, but got none")
	}

	// Check that the error message indicates Docker is required
	if !strings.Contains(err.Error(), "docker is not available") {
		t.Errorf("expected error to mention Docker not available, got: %v", err)
	}
}

func TestWriteFromSshDocker(t *testing.T) {
	// This test would require Docker to be installed in the test container
	// For now, we'll skip it as the main focus is testing the basic SSH+Docker logic
	t.Skip("SSH+Docker test requires Docker to be installed in test container")
}

func TestNewSshDocker(t *testing.T) {
	cfg := SshDockerCfg{
		ContainerName: "test-container",
		BinPath:       "/usr/bin/pg_dump",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewSshDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.containerName != "test-container" {
		t.Errorf("expected container name test-container, got: %s", h.containerName)
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

func TestNewSshDockerDefaultBinPath(t *testing.T) {
	cfg := SshDockerCfg{
		ContainerName: "test-container",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewSshDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "pg_dump" {
		t.Errorf("expected default bin path pg_dump, got: %s", h.binPath)
	}
}

func TestSshDockerCmd(t *testing.T) {
	h := SshDockerHandler{
		binPath:       "/usr/bin/pg_dump",
		containerName: "test-container",
		user:          "testuser",
		pw:            "testpass",
		dbName:        "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump -U testuser -W --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

func TestSshDockerCmdNoUser(t *testing.T) {
	h := SshDockerHandler{
		binPath:       "/usr/bin/pg_dump",
		containerName: "test-container",
		user:          "",
		pw:            "",
		dbName:        "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}
