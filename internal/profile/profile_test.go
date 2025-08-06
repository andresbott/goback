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
			file: "sampledata/correctProfileDir/local.backup.yaml",
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
			file: "sampledata/correctProfileDir/remote.backup.yaml",
			want: Profile{
				Name: "remote",
				Type: TypeRemote,
				Ssh: Ssh{
					Type:       ConnTypePasswd,
					Host:       "bla.ble.com",
					Port:       2222,
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
			file: "sampledata/correctProfileDir/sftpSync.backup.yaml",
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
					{Path: "/backup/service1", Name: "service1"},
					{Path: "/backup/service2", Name: "service2"},
				},
				Destination: Destination{
					Path:  "/backups",
					Keep:  3,
					Owner: "ble",
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
		name      string
		file      string
		wantError string
	}{
		{
			name:      "Load from wrong extension",
			file:      "sampledata/json.json",
			wantError: "profile path is not a .yaml file",
		},
		{
			name:      "Missing name in profile",
			file:      "sampledata/errCases/missingName.yaml",
			wantError: "profile name cannot be empty",
		},
		{
			name:      "file not found",
			file:      "sampledata/doesnotexist.yaml",
			wantError: "no such file or directory",
		},
		{
			name:      "malformed YAML",
			file:      "sampledata/errCases/malformed.yaml",
			wantError: "yaml",
		},
		{
			name:      "missing required field (type)",
			file:      "sampledata/errCases/missing_type.yaml",
			wantError: "profile has no type",
		},
		{
			name:      "invalid backup type",
			file:      "sampledata/errCases/invalid_type.yaml",
			wantError: "invalid type",
		},
		{
			name:      "invalid glob pattern in exclude",
			file:      "sampledata/errCases/invalid_glob.yaml",
			wantError: "unable to compile exclude pattern",
		},
		{
			name:      "invalid port (non-numeric)",
			file:      "sampledata/errCases/invalid_port.yaml",
			wantError: "cannot unmarshal",
		},
		{
			name:      "missing version",
			file:      "sampledata/errCases/missing_version.yaml",
			wantError: "unsupported profile version: 0",
		},
		{
			name:      "invalid remote connection type",
			file:      "sampledata/errCases/invalid_remote.yaml",
			wantError: "profile has invalid ssh connection type",
		},
		{
			name:      "invalid backup content",
			file:      "sampledata/errCases/invalid_backup_content.yaml",
			wantError: "nothing to backup",
		},
		{
			name:      "missing sync path name",
			file:      "sampledata/errCases/missing_sync_path_name.yaml",
			wantError: "profile name for sync path cannot be empty",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadProfile(tc.file)
			if err == nil {
				t.Fatal("expecting an error but returned nil")
			}

			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error to contain %q, got: %v", tc.wantError, err)
			}
		})
	}
}

func TestLoadProfiles(t *testing.T) {

	t.Run("load directory with profiles", func(t *testing.T) {
		profiles, err := LoadProfiles("sampledata/correctProfileDir")
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

	t.Run("expect partial list with error profiles", func(t *testing.T) {
		profiles, err := LoadProfiles("sampledata/errProfileDir")
		wantErrs := []string{
			"failed to load profile sampledata/errProfileDir/invalid.backup.yaml: profile has invalid ssh connection type",
			"failed to load profile sampledata/errProfileDir/invalid_port.backup.yaml: yaml: unmarshal errors",
		}

		// Type assert Unwrap() []error
		if unwrapped, ok := err.(interface{ Unwrap() []error }); ok {
			for _, wantErr := range wantErrs {
				foundErr := false
				for _, gotErr := range unwrapped.Unwrap() {
					if strings.Contains(gotErr.Error(), wantErr) {
						foundErr = true
						break
					}
				}
				if !foundErr {
					t.Errorf("expected error to contain %s", wantErr)
				}
			}
		} else {
			t.Fatalf("expect an unwrapped error type, got: %T", unwrapped)
		}

		got := []string{}
		for _, p := range profiles {
			got = append(got, p.Name)
		}
		want := []string{
			"localBackup",
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
		_, err := LoadProfiles("sampledata/correctProfileDir/local.backup.yaml")
		if err == nil {
			t.Fatal("expecting an error but none was returned")
		}

		expectedErr := "the path is not a directory"
		if err.Error() != expectedErr {
			t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
		}
	})
}
