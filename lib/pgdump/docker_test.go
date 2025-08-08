package pgdump

import (
	"testing"
)

func TestWriteFromDocker(t *testing.T) {
	// This test would require a real Docker client, so we'll skip it for now
	// In a real implementation, you'd need to mock the Docker client or have a test container
	t.Skip("Docker test requires real Docker client")
}

func TestNewDocker(t *testing.T) {
	cfg := DockerCfg{
		ContainerName: "test-container",
		BinPath:       "/usr/bin/pg_dump",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewDocker(cfg)
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

	if h.client == nil {
		t.Error("expected Docker client to be initialized")
	}

	err = h.Close()
	if err != nil {
		t.Errorf("unexpected error closing client: %v", err)
	}
}

func TestNewDockerDefaultBinPath(t *testing.T) {
	cfg := DockerCfg{
		ContainerName: "test-container",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "pg_dump" {
		t.Errorf("expected default bin path pg_dump, got: %s", h.binPath)
	}

	err = h.Close()
	if err != nil {
		t.Errorf("unexpected error closing client: %v", err)
	}
}

func TestDockerCmd(t *testing.T) {
	h := DockerHandler{
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

func TestDockerCmdNoUser(t *testing.T) {
	h := DockerHandler{
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

func TestDockerClientMethods(t *testing.T) {
	cfg := DockerCfg{
		ContainerName: "test-container",
		BinPath:       "/usr/bin/pg_dump",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer h.Close()

	client := h.DockerClient()
	if client == nil {
		t.Error("expected Docker client to be returned")
	}

	containerName := h.ContainerName()
	if containerName != "test-container" {
		t.Errorf("expected container name test-container, got: %s", containerName)
	}

	h.SetBinPath("/new/path/pg_dump")
	if h.binPath != "/new/path/pg_dump" {
		t.Errorf("expected bin path to be updated to /new/path/pg_dump, got: %s", h.binPath)
	}
}
