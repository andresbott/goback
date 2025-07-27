package goback

import (
	"archive/zip"
	"bytes"
	"io"
	"log"
	"os"
	"testing"

	"github.com/AndresBott/goback/internal/profile"
	zipHandler "github.com/AndresBott/goback/lib/zip"
	"github.com/google/go-cmp/cmp"
)

func TestDumpDb(t *testing.T) {

	setup := func(t *testing.T) (string, *zipHandler.Handler, func(t *testing.T)) {
		dir, err := os.MkdirTemp("", "TestDumpDb_")
		if err != nil {
			log.Fatal(err)
		}

		zipFile := dir + "/test_zip.zip"

		zh, err := zipHandler.New(zipFile)
		if err != nil {
			log.Fatal(err)
		}

		// destructor function
		destructor := func(t *testing.T) {
			os.RemoveAll(dir)
		}

		return zipFile, zh, destructor

	}

	mockBin := "./sampledata/mysqldump"
	zipFile, zh, destroy := setup(t)
	defer destroy(t)

	in := profile.MysqlBackup{
		DbName: "testDbName",
		User:   "user",
		Pw:     "pass",
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

// TODO uncoment
//func TestDumpSshDb(t *testing.T) {
//	skipInCI(t) // skip test if running in CI
//
//	ctx := context.Background()
//	sshServer, err := setupContainer(ctx)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer sshServer.Terminate(ctx)
//
//	setup := func(t *testing.T) (string, *zipHandler.Handler, func(t *testing.T)) {
//		dir, err := os.MkdirTemp("", "TestDumpSshDb_")
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		zipFile := dir + "/test_zip.zip"
//
//		zh, err := zipHandler.New(zipFile)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// destructor function
//		destructor := func(t *testing.T) {
//			os.RemoveAll(dir)
//		}
//
//		return zipFile, zh, destructor
//
//	}
//
//	zipFile, zh, destroy := setup(t)
//	defer destroy(t)
//
//	in := profile.MysqlBackup{
//		DbName: "testDbName",
//		User:   "user",
//		Pw:     "pass",
//	}
//
//	cl, err := ssh.New(ssh.Cfg{
//		Host:          sshServer.host,
//		Port:          sshServer.port,
//		Auth:          ssh.Password,
//		User:          "pwuser",
//		Password:      "1234",
//		IgnoreHostKey: true,
//	})
//	if err != nil {
//		t.Fatalf("unexpected error %v", err)
//	}
//
//	cl.Connect()
//
//	err = dumpSshDb(cl, "/usr/local/bin/mysqldump", in, zh)
//	if err != nil {
//		t.Fatalf("unexpected error: %v", err)
//	}
//	zh.Close()
//
//	got, err := listFilesInZip(zipFile)
//	if err != nil {
//		t.Fatalf("unexpected error: %v", err)
//	}
//
//	expect := []string{
//		"_mysqldump/testDbName.dump.sql",
//	}
//	if diff := cmp.Diff(expect, got); diff != "" {
//		t.Errorf("output mismatch (-want +got):\n%s", diff)
//	}
//
//	expectContent := "mysqldump mock binary, params: -u user -ppass --add-drop-database --databases testDbName\n"
//	gotContent, err := readFileInZip(zipFile, "_mysqldump/testDbName.dump.sql")
//	if err != nil {
//		t.Fatalf("unexpected error: %v", err)
//	}
//
//	if diff := cmp.Diff(expectContent, gotContent); diff != "" {
//		t.Errorf("output mismatch (-want +got):\n%s", diff)
//	}
//}

func readFileInZip(zipFile string, file string) (string, error) {

	zf, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}
	defer zf.Close()

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
		_, err = io.Copy(&buf, rc)
		if err != nil {
			return "", err
		}
		rc.Close()
		break
	}

	return buf.String(), nil
}
