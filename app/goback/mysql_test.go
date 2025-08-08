package goback

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log"
	"testing"

	"github.com/AndresBott/goback/lib/ssh"

	"github.com/AndresBott/goback/internal/profile"
	zipHandler "github.com/AndresBott/goback/lib/zip"
	"github.com/google/go-cmp/cmp"
)

func TestCopyMysql_local(t *testing.T) {

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
	mockBin := "./sampledata/mysqldump"

	in := profile.BackupDb{
		Name:     "testDbName",
		User:     "user",
		Password: "pass",
	}

	err := copyLocalMysql(mockBin, in, zh)
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

func TestCopyMysql_remote(t *testing.T) {
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

	in := profile.BackupDb{
		Name:     "testDbName",
		User:     "user",
		Password: "pass",
	}

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

	err = copyRemoteMysql(cl, "/usr/local/bin/mysqldump", in, zh)
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

func TestCopyMysql_docker(t *testing.T) {
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

	dbPrfl := profile.BackupDb{
		Name:     "testDbName",
		User:     "user",
		Password: "pass",
	}

	// Get the container ID from the sshServer
	containerID := sshServer.GetContainerID()

	err = copyLocalDockerMysql(containerID, dbPrfl, zh)
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

func readFileInZip(zipFile string, file string) (string, error) {

	zf, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = zf.Close()
	}()

	buf := bytes.Buffer{}
	for _, f := range zf.File {
		if f.Name != file {
			continue
		}

		// Found it, print its content to terminal:
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		// #nosec G110 -- test code is controlled
		_, err = io.Copy(&buf, rc)
		if err != nil {
			return "", err
		}
		_ = rc.Close()
		break
	}

	return buf.String(), nil
}
