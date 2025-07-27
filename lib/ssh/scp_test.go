package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//nolint:gocyclo //accepted in this, test added before linter rule
func TestScpActions(t *testing.T) {
	//skipInCI(t) // skip test if running in CI

	// since git will drop empty folders, we create if as part of the test
	if _, err := os.Stat("./sampledata/dir/empty"); os.IsNotExist(err) {
		//#nosec G301 - permissions needed for test
		err := os.Mkdir("./sampledata/dir/empty", os.ModePerm)
		if err != nil {
			t.Fatalf("unable to create empty folder")
		}
	}

	defer func() {
		err := os.RemoveAll("./sampledata/dir/empty")
		if err != nil {
			t.Fatalf("unable to delte empty folder used in test")
		}
	}()

	// in order to minimize the start/stop time of testconainers we run one setup and every subsequent test
	// us executed as a sub tests
	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

	t.Run("test copy from remote", func(t *testing.T) {
		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer func() {
			_ = cl.Disconnect()
		}()

		scpc, err := NewScp(cl.conn)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		ctx := context.Background()
		var got bytes.Buffer

		err = scpc.CopyFromRemotePassThru(ctx, &got, "/data/testfile.txt", nil)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		want := "testfile"

		if diff := cmp.Diff(want, got.String()); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Test Stat on file", func(t *testing.T) {

		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer func() {
			_ = cl.Disconnect()
		}()

		t.Run("assert a directory", func(t *testing.T) {
			got, err := cl.Stat("/data/dir with&$/subdir")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := FileStat{
				isDir: true,
				name:  "subdir",
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("assert a file", func(t *testing.T) {

			got, err := cl.Stat("/data/testfile.txt")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := FileStat{
				isDir: false,
				name:  "testfile.txt",
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})

	})

	t.Run("TestReadDir", func(t *testing.T) {

		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer func() {
			_ = cl.Disconnect()
		}()

		t.Run("regular dir", func(t *testing.T) {
			got, err := cl.ReadDir("/data/dir with&$")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := []FileStat{
				{
					isDir: false,
					name:  "testfile.txt",
				},
				{
					isDir: false,
					name:  "file.json",
				},
				{
					isDir: true,
					name:  "subdir",
				},
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("empty dir", func(t *testing.T) {
			got, err := cl.ReadDir("/data/empty")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := []FileStat{}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("non existent path", func(t *testing.T) {
			got, err := cl.ReadDir("/data/nonexistent")
			if err == nil {
				t.Fatalf("expected error but got %v", got)
			}
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected ErrNotExist but got %v", err)
			}
		})

		t.Run("not a dir", func(t *testing.T) {
			got, err := cl.ReadDir("/data/testfile.txt")
			if err == nil {
				t.Fatalf("expected error but got %v", got)
			}
			if !errors.Is(err, ErrNotADir) {
				t.Fatalf("expected ErrNotADir but got %v", err)
			}
		})

	})

	t.Run("WalkDir", func(t *testing.T) {
		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer func() {
			_ = cl.Disconnect()
		}()

		var got []string
		fn := func(path string, info FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error in path: \"%s\", %v\n", path, err)
			}
			got = append(got, fmt.Sprintf("d:%v - %s", info.IsDir(), path))
			//fmt.Printf("path: %s - name: %s, isdir: %v\n", path, info.Name(), info.IsDir())
			return nil
		}

		err = cl.WalkDir("/data", fn)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		want := []string{
			"d:true - /data",
			"d:true - /data/dir with&$",
			"d:false - /data/dir with&$/file.json",
			"d:true - /data/dir with&$/subdir",
			"d:false - /data/dir with&$/subdir/.gitkeep",
			"d:false - /data/dir with&$/testfile.txt",
			"d:true - /data/empty", // created at the top of the test
			"d:true - /data/files",
			"d:true - /data/files/dir1",
			"d:false - /data/files/dir1/file.json",
			"d:true - /data/files/dir1/subdir1",
			"d:false - /data/files/dir1/subdir1/subfile.log",
			"d:false - /data/files/dir1/subdir1/subfile1.txt",
			"d:true - /data/files/dir2",
			"d:false - /data/files/dir2/file.yaml",
			"d:false - /data/testfile.txt",
		}
		sort.Strings(want)
		sort.Strings(got)

		if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}

	})
}

func TestParseLsLine(t *testing.T) {
	tcs := []struct {
		name   string
		in     string
		expect FileStat
	}{
		{
			name: "regular file",
			in:   "regular file_/data/dir with&$/testfile.txt\n",
			expect: FileStat{
				isDir: false,
				name:  "testfile.txt",
			},
		},
		{
			name: "directory",
			in:   "directory_datos/wine\n",
			expect: FileStat{
				isDir: true,
				name:  "wine",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStatLine(tc.in)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := tc.expect
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}

}
