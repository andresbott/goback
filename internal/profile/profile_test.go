package profile

import (
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
	"strings"
	"testing"
)

func getGlob(in string) glob.Glob {
	return glob.MustCompile(in)
}

func TestLoadProfile(t *testing.T) {

	tcs := []struct {
		name string
		file string
		want Profile
	}{
		{
			name: "local backup",
			file: "sampledata/local.backup.yaml",
			want: Profile{
				Name: "localBackup",
				Type: TypeLocal,
				Dirs: []BackupPath{
					{
						Path: "/bla",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
					{
						Path: "/ble",
						Exclude: []glob.Glob{
							getGlob("*.logs"),
						},
					},
				},
				Dbs: []BackupDb{
					{
						Name:     "dbname",
						User:     "user",
						Password: "pw",
						Type:     DbMysql,
					},
				},
				Destination: Destination{
					Path:  "/backups",
					Keep:  3,
					Owner: "ble",
					Group: "ble",
					Mode:  "0600",
				},
				Notify: EmailNotify{
					Host:     "smtp.mail.com",
					Port:     "587",
					User:     "mail@mails.com",
					Password: "1234",
					To:       []string{"mail1@mail.com", "mail2@mail.com"},
				},
			},
		},

		{
			name: "profile with remote configuration",
			file: "sampledata/remote.backup.yaml",
			want: Profile{
				Name: "remote",
				Type: TypeRemote,
				Ssh: Ssh{
					Type:       ConnTypePasswd,
					Host:       "bla.ble.com",
					Port:       22,
					User:       "user",
					Password:   "bla",
					PrivateKey: "privKey",
					Passphrase: "pass",
				},
				Dirs: []BackupPath{
					{
						Path: "relative/path",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
					{
						Path: "/backup/service2",
					},
				},
				Dbs: []BackupDb{
					{
						Name:     "dbname",
						User:     "user",
						Password: "pw",
						Type:     "mysql",
					},
				},
				Destination: Destination{
					Path:  "/backups",
					Keep:  3,
					Owner: "ble",
					Group: "ble",
					Mode:  "0600",
				},
				Notify: EmailNotify{
					Host:     "smtp.mail.com",
					Port:     "587",
					User:     "mail@mails.com",
					Password: "1234",
					To:       []string{"mail1@mail.com", "mail2@mail.com"},
				},
			},
		},

		{
			name: "profile with sftp sync",
			file: "sampledata/sftpSync.backup.yaml",
			want: Profile{
				Name: "sftpSync",
				Type: TypeSftpSync,
				Ssh: Ssh{
					Type:       ConnTypeSshKey,
					Host:       "bla.ble.com",
					Port:       22,
					PrivateKey: "/path/To/key",
					Passphrase: "pass",
				},
				Dirs: []BackupPath{
					{Path: "/backup/service1"},
					{Path: "/backup/service2"},
				},
				Destination: Destination{
					Path:  "/backups",
					Keep:  3,
					Owner: "ble",
					Group: "ble",
					Mode:  "0600",
				},
				Notify: EmailNotify{
					Host:     "smtp.mail.com",
					Port:     "587",
					User:     "mail@mails.com",
					Password: "1234",
					To:       []string{"mail1@mail.com", "mail2@mail.com"},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoadProfile(tc.file)
			if err != nil {
				t.Fatal("unexpected error: ", err)
			}

			want := tc.want
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("output mismatch (-got +want):\n%s", diff)
			}
		})
	}

	tcsErr := []struct {
		name      string
		file      string
		wantError string
	}{
		{
			name:      "file not found",
			file:      "sampledata/doesnotexist.yaml",
			wantError: "no such file or directory",
		},
		{
			name:      "malformed YAML",
			file:      "sampledata/errCases/malformed.yaml", // e.g. missing colons or incorrect indentation
			wantError: "yaml",
		},
		{
			name:      "missing required field (type)",
			file:      "sampledata/errCases/missing_type.yaml", // omits the 'type' field
			wantError: "profile has no type",
		},
		{
			name:      "invalid backup type",
			file:      "sampledata/errCases/invalid_type.yaml", // e.g. `type: unknownType`
			wantError: "invalid type",
		},
		{
			name:      "invalid glob pattern in exclude",
			file:      "sampledata/errCases/invalid_glob.yaml", // e.g. `exclude: [ "**[.log" ]`
			wantError: "unable to compile exclude pattern",
		},
		{
			name:      "invalid port (non-numeric)",
			file:      "sampledata/errCases/invalid_port.yaml", // e.g. `port: abc`
			wantError: "cannot unmarshal",
		},
		{
			name:      "missing version",
			file:      "sampledata/errCases/missing_version.yaml", // e.g. `port: abc`
			wantError: "unsupported profile version: 0",
		},
	}

	for _, tc := range tcsErr {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadProfile(tc.file)
			if err == nil {
				t.Fatal("expected an error but got nil")
			}

			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error to contain %q, got: %v", tc.wantError, err)
			}
		})
	}

	t.Run("load nonexistent file", func(t *testing.T) {
		_, err := LoadProfile("sampledata/nonexistent.yaml")
		if err == nil {
			t.Fatal("expecting an error but none was returned")
		}

		expectedErr := "open sampledata/nonexistent.yaml: no such file or directory"
		if err.Error() != expectedErr {
			t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
		}
	})

	t.Run("load from wrong file type", func(t *testing.T) {
		_, err := LoadProfile("sampledata/json.json")
		if err == nil {
			t.Fatal("expecting an error but none was returned")
		}

		expectedErr := "profile path is not a .yaml file"
		if err.Error() != expectedErr {
			t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
		}
	})

}

func TestLoadProfileErrors(t *testing.T) {

	tcs := []struct {
		name string
		file string
		want string
	}{
		{
			name: "Load from wrong extension",
			file: "sampledata/json.json",
			want: "profile path is not a .yaml file",
		},

		{
			name: "File does not exitss",
			file: "sampledata/inexistent.yaml",
			want: "open sampledata/inexistent.yaml: no such file or directory",
		},
		{
			name: "Missing name in profile",
			file: "sampledata/missingName.yaml",
			want: "profile name cannot be empty",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadProfile(tc.file)
			if err == nil {
				t.Fatal("expecting an error but returned nil")
			}

			got := err.Error()

			want := tc.want
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("output mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestLoadProfiles(t *testing.T) {

	t.Run("load directory with profiles", func(t *testing.T) {
		profiles, err := LoadProfiles("sampledata")
		if err != nil {
			t.Fatal("umexpected error: ", err)
		}

		got := []string{}
		for _, p := range profiles {
			got = append(got, p.Name)
		}
		want := []string{
			"localBackup",
			"remote",
			"sftpSync",
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("output mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("expect error on nonexixtent dir", func(t *testing.T) {
		_, err := LoadProfiles("nonexistent")
		if err == nil {
			t.Fatal("expecting an error but none was returned")
		}

		expectedErr := "stat nonexistent: no such file or directory"
		if err.Error() != expectedErr {
			t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
		}
	})

	t.Run("expect error on file instead of dir", func(t *testing.T) {
		_, err := LoadProfiles("sampledata/local.backup.yaml")
		if err == nil {
			t.Fatal("expecting an error but none was returned")
		}

		expectedErr := "the path is not a directory"
		if err.Error() != expectedErr {
			t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
		}
	})
}
