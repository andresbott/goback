package ssh

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScpActions(t *testing.T) {
	// in order to minimize the start/stop time of testconainers we run one setup and every subsequent test
	// us executed as a sub tests
	ctx := context.Background()

	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sshServer.Terminate(ctx)

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
		defer cl.Disconnect()

		scpc, err := NewScp(cl.conn)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		ctx := context.Background()
		var got bytes.Buffer

		err = scpc.Client.CopyFromRemotePassThru(ctx, &got, "/data/testfile.txt", nil)
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
		defer cl.Disconnect()

		t.Run("assert a directory", func(t *testing.T) {
			got, err := cl.Stat("/data/dir with&$/empty dir")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := FileStat{
				isDir: true,
				name:  "empty dir",
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

	t.Run("Test Readdir", func(t *testing.T) {

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
		defer cl.Disconnect()

		t.Run("regular dir", func(t *testing.T) {
			got, err := cl.ReadDir("/data/dir with&$")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := []FileStat{
				{
					isDir: true,
					name:  "empty dir",
				},
				{
					isDir: false,
					name:  "file.json",
				},
				{
					isDir: false,
					name:  "testfile.txt",
				},
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("empty dir", func(t *testing.T) {
			got, err := cl.ReadDir("/data/dir with&$/empty dir")
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			want := []FileStat{}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(FileStat{})); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
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
		defer cl.Disconnect()

		got := []string{}

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
			"d:true - /data/dir with&$/empty dir",
			"d:false - /data/dir with&$/file.json",
			"d:false - /data/dir with&$/testfile.txt",
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
