package mysqldump

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
	zipWriter, err := zh.FileWriter(filepath.Join("_mysqldump", "testDbName.dump.sql"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := LocalCfg{
		BinPath: "./sampledata/local/mock_mysqldump.sh",
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

func TestReadIni(t *testing.T) {

	const expectedUsr = "usr"
	const expectedPw = "pw"

	verifyResult := func(user string, pw string, t *testing.T) {
		if user != expectedUsr {
			t.Errorf("value for user is unexpected, got \"%s\", want \"%s\"", user, expectedUsr)
		}
		if pw != expectedPw {
			t.Errorf("value for passwod is unexpected, got \"%s\", want \"%s\"", pw, expectedPw)
		}
	}

	t.Run("read values from file", func(t *testing.T) {
		h := LocalHandler{}
		err := h.loadCnfFiles([]string{
			"sampledata/local/my3.cnf",
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}
		verifyResult(h.user, h.pw, t)
	})

	t.Run("verify overlay", func(t *testing.T) {
		h := LocalHandler{}
		err := h.loadCnfFiles([]string{
			"sampledata/local/my1.cnf",
			"sampledata/local/my2.cnf",
			"sampledata/local/my3.cnf",
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}
		verifyResult(h.user, h.pw, t)
	})

	t.Run("verify all files even if non existent", func(t *testing.T) {
		h := LocalHandler{}
		err := h.loadCnfFiles([]string{
			"sampledata/local/doesNotExist.cnf",
			"sampledata/local/my3.cnf",
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}
		verifyResult(h.user, h.pw, t)
	})
}

func TestGetCmd(t *testing.T) {

	t.Run("get args with user and password", func(t *testing.T) {
		args := getArgs("myUser", "mypw", "dbName")

		argsWant := []string{
			"-u",
			"myUser",
			"-pmypw",
			"--add-drop-database",
			"--databases",
			"dbName",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("get args without user and password", func(t *testing.T) {
		args := getArgs("", "", "dbName")

		argsWant := []string{
			"--add-drop-database",
			"--databases",
			"dbName",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("get args with user only", func(t *testing.T) {
		args := getArgs("myUser", "", "dbName")

		argsWant := []string{
			"-u",
			"myUser",
			"--add-drop-database",
			"--databases",
			"dbName",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("get args with password only", func(t *testing.T) {
		args := getArgs("", "mypw", "dbName")

		argsWant := []string{
			"-pmypw",
			"--add-drop-database",
			"--databases",
			"dbName",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("get args with special characters in strings", func(t *testing.T) {
		args := getArgs("my User", "my pw\n", "my db\r")

		argsWant := []string{
			"-u",
			"my_User",
			"-pmy_pw",
			"--add-drop-database",
			"--databases",
			"my_db",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("get args with empty database name", func(t *testing.T) {
		args := getArgs("user", "pw", "")

		argsWant := []string{
			"-u",
			"user",
			"-ppw",
			"--add-drop-database",
			"--databases",
			"",
		}
		if diff := cmp.Diff(argsWant, args); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})
}

// this test executes a mock mysqldump binary and compares the printed output of that binary to the expected value
func TestExecute(t *testing.T) {
	myh := LocalHandler{
		binPath: "./sampledata/local/mock_mysqldump.sh",
		dbName:  "dbName",
		user:    "myUser",
		pw:      "mypw",
	}

	var buf bytes.Buffer

	err := myh.Run(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	// the sample script prints all the parameters as seen
	want := "mysqldump mock binary, params: -u myUser -pmypw --add-drop-database --databases dbName\n"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

// this test forces the mock mysqldump to exit with error code 1 and expect the error to be propagated
func TestFailedExecution(t *testing.T) {
	myh := LocalHandler{
		binPath: "./sampledata/local/mock_mysqldump.sh",
		dbName:  "dbName",
		user:    "fail",
		pw:      "mypw",
	}

	var buf bytes.Buffer

	err := myh.Run(&buf)
	want := "error running mysqldump: exit status 1"
	if err.Error() != want {
		t.Fatalf("expecting error:\"%s\" but got \"%v\"  ", want, err)
	}

}
