package goback

import (
	"testing"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
)

type fileAppender struct {
	files []string
}

func (a *fileAppender) AddFile(origin string, dest string) error {
	a.files = append(a.files, dest)
	return nil
}
func (a *fileAppender) AddSymlink(origin string, dest string) error {
	a.files = append(a.files, dest)
	return nil
}

func TestCopyLocalFiles(t *testing.T) {

	tcs := []struct {
		name    string
		profile profile.BackupPath
		want    []string
	}{
		{
			name: "expect all files processed correctly",
			profile: profile.BackupPath{
				Path:    "sampledata/files",
				Exclude: nil,
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir1/subdir1/subfile.log",
				"files/dir1/subdir1/subfile1.txt",
				"files/dir2/.hidden",
				"files/dir2/file.yaml",
				"files/notRoot/link",
			},
		},

		{
			name: "expect files to be filtered out, multiple filters",
			profile: profile.BackupPath{
				Path: "sampledata/files",
				Exclude: []glob.Glob{
					getGlob("*.log"),
					getGlob("*.txt"),
				},
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir2/.hidden",
				"files/dir2/file.yaml",
				"files/notRoot/link",
			},
		},

		{
			name: "expect full directory to be excluded",
			profile: profile.BackupPath{
				Path: "sampledata/files",
				Exclude: []glob.Glob{
					getGlob("sampledata/files/dir1/subdir1/*"),
				},
			},
			want: []string{
				"files/dir1/file.json",
				"files/dir2/.hidden",
				"files/dir2/file.yaml",
				"files/notRoot/link",
			},
		},
		{
			name: "expect root symlink evaluated and backed up",
			profile: profile.BackupPath{
				Path: "sampledata/files/notRoot/link",
				Exclude: []glob.Glob{
					getGlob("*.txt"),
				},
			},
			want: []string{
				"link/file.json",
				"link/subdir1/subfile.log",
			},
		},
		{
			name: "don't follow child symlinks",
			profile: profile.BackupPath{
				Path: "sampledata/files/",
				Exclude: []glob.Glob{
					getGlob("sampledata/files/dir1/*"),
					getGlob("sampledata/files/dir2/*"),
				},
			},
			want: []string{
				"files/notRoot/link",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			fa := fileAppender{}
			err := copyLocalFiles(tc.profile, &fa)
			got := fa.files

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}

		})
	}
}

func TestCopyRemoteFiles(t *testing.T) {

}
