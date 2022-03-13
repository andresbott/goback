package mysqldump

import (
	"bytes"
	"github.com/google/go-cmp/cmp"
	"testing"
)

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
		h := Handler{}
		err := h.userFromCnf([]string{
			"sampledata/my3.cnf",
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}
		verifyResult(h.user, h.pw, t)
	})

	t.Run("verify overlay", func(t *testing.T) {
		h := Handler{}
		err := h.userFromCnf([]string{
			"sampledata/my1.cnf",
			"sampledata/my2.cnf",
			"sampledata/my3.cnf",
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}
		verifyResult(h.user, h.pw, t)
	})

	t.Run("verify error is returned", func(t *testing.T) {
		h := Handler{}
		err := h.userFromCnf([]string{
			"sampledata/my1.cnf",
		})
		if err == nil {
			t.Errorf("expeciting error but none returned")
		}

		if err.Error() != "user or password is empty" {
			t.Errorf("got unexpected error message: %s", err.Error())
		}

	})
}

func TestGetCmd(t *testing.T) {

	myh := Handler{
		binPath: "/bin/myd",
		dbName:  "dbName",
		user:    "myUser",
		pw:      "mypw",
	}

	bin, args := myh.getCmd()

	binWant := "/bin/myd"
	if bin != binWant {
		t.Errorf("unexpected bin path, got: %s, want: %s", bin, binWant)
	}
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

}

// this test executes a mock mysqldump binary and compares the printed output of that binary to the expected value
func TestExecute(t *testing.T) {
	myh := Handler{
		binPath: "./sampledata/mock_mysqldump.sh",
		dbName:  "dbName",
		user:    "myUser",
		pw:      "mypw",
	}

	var buf bytes.Buffer

	err := myh.Exec(&buf)
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
	myh := Handler{
		binPath: "./sampledata/mock_mysqldump.sh",
		dbName:  "dbName",
		user:    "fail",
		pw:      "mypw",
	}

	var buf bytes.Buffer

	err := myh.Exec(&buf)
	want := "error running mysqldump: exit status 1"
	if err.Error() != want {
		t.Fatalf("expecting error:\"%s\" but got \"%v\"  ", want, err)
	}

}
