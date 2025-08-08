package pgdump

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	zipHandler "github.com/AndresBott/goback/lib/zip"
	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWriteFromDocker(t *testing.T) {
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

	zipWriter, err := zh.FileWriter(filepath.Join("_pgdump", "testDbName.dump.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// Get the container ID from the sshServer
	cfg := DockerCfg{
		ContainerName: sshServer.name,
		User:          "user",
		Pw:            "pass",
		DbName:        "testDbName",
	}

	err = WriteFromDocker(t.Context(), cfg, zipWriter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	zh.Close()

	got, err := listFilesInZip(zipFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expect := []string{
		"_pgdump/testDbName.dump.sql",
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}

	expectContent := "pg_dump mock binary, params: -U user -W --clean --if-exists --create --verbose testDbName\n"
	gotContent, err := readFileInZip(zipFile, "_pgdump/testDbName.dump.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func skipInCI(t *testing.T) {
	envSet := os.Getenv("RUN_TESTCONTAINERS")
	if envSet == "" {
		t.Skip("skipping because env \"RUN_TESTCONTAINERS\" is not set to true")
	}
}

type sshContainer struct {
	testcontainers.Container
	name string
	host string
	port int
}

func setupContainer(ctx context.Context) (*sshContainer, error) {

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "./sampledata/docker",
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

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "22")
	if err != nil {
		return nil, err
	}

	name, err := container.Name(ctx)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(mappedPort.Port())

	sshCont := &sshContainer{
		Container: container,
		name:      name,
		host:      ip,
		port:      port,
	}

	return sshCont, nil
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
