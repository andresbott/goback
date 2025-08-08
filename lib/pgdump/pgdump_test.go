package pgdump

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"testing"

	zipHandler "github.com/AndresBott/goback/lib/zip"
	"github.com/google/go-cmp/cmp"
)

func TestWriteLocal(t *testing.T) {

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

	cfg := LocalCfg{
		BinPath: "./sampledata/local/mock_pg_dump.sh",
		User:    "user",
		Pw:      "pass",
		DbName:  "testDbName",
	}

	err = WriteLocal(cfg, zipWriter)
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

func listFilesInZip(in string) ([]string, error) {
	read, err := zip.OpenReader(in)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %s", err)
	}
	defer func() {
		_ = read.Close()
	}()

	files := []string{}
	for _, file := range read.File {
		files = append(files, file.Name)
	}
	return files, nil
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
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer func() {
			_ = rc.Close()
		}()
		// #nosec G110 -- test code is controlled
		_, err = io.Copy(&buf, rc)
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	return "", fmt.Errorf("file not found: %s", file)
}

func TestReadIni(t *testing.T) {
	// Test with non-existent file
	h := LocalHandler{}
	err := h.loadCnfFiles([]string{"/non/existent/file"})
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}

	// Test with empty user/pw
	if h.user != "" || h.pw != "" {
		t.Errorf("expected empty user/pw, got user: %s, pw: %s", h.user, h.pw)
	}
}

func TestGetCmd(t *testing.T) {
	h := LocalHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "testuser",
		pw:      "testpass",
		dbName:  "testdb",
	}

	bin, args := h.Cmd()
	if bin != "/usr/bin/pg_dump" {
		t.Errorf("expected bin path /usr/bin/pg_dump, got: %s", bin)
	}

	expectedArgs := []string{"-U", "testuser", "-W", "--clean", "--if-exists", "--create", "--verbose", "testdb"}
	if diff := cmp.Diff(expectedArgs, args); diff != "" {
		t.Errorf("args mismatch (-want +got):\n%s", diff)
	}
}

func TestGetCmdNoUser(t *testing.T) {
	h := LocalHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "",
		pw:      "",
		dbName:  "testdb",
	}

	bin, args := h.Cmd()
	if bin != "/usr/bin/pg_dump" {
		t.Errorf("expected bin path /usr/bin/pg_dump, got: %s", bin)
	}

	expectedArgs := []string{"--clean", "--if-exists", "--create", "--verbose", "testdb"}
	if diff := cmp.Diff(expectedArgs, args); diff != "" {
		t.Errorf("args mismatch (-want +got):\n%s", diff)
	}
}

func TestExecute(t *testing.T) {
	h := LocalHandler{
		binPath: "./sampledata/local/mock_pg_dump.sh",
		user:    "user",
		pw:      "pass",
		dbName:  "testDbName",
	}

	var buf bytes.Buffer
	err := h.Run(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "pg_dump mock binary, params: -U user -W --clean --if-exists --create --verbose testDbName\n"
	if buf.String() != expected {
		t.Errorf("expected output: %s, got: %s", expected, buf.String())
	}
}

func TestFailedExecution(t *testing.T) {
	h := LocalHandler{
		binPath: "/non/existent/pg_dump",
		user:    "user",
		pw:      "pass",
		dbName:  "testDbName",
	}

	var buf bytes.Buffer
	err := h.Run(&buf)
	if err == nil {
		t.Fatal("expected error for non-existent binary")
	}
}
