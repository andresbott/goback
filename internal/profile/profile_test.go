package profile

import (
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
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
				Name: "test",
				Dirs: []BackupDir{
					{
						Root: "/bla",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
					{
						Root: "/ble",
						Exclude: []glob.Glob{
							getGlob("*.logs"),
						},
					},
				},
				Mysql: []MysqlBackup{
					{
						DbName: "dbname",
						User:   "user",
						Pw:     "pw",
					},
				},
				Destination: "/backups",
				Keep:        3,
				Owner:       "ble",
				Mode:        "0700",
			},
		},

		{
			name: "profile with remote configuration",
			file: "sampledata/remote.backup.yaml",
			want: Profile{
				Name:     "remote",
				IsRemote: true,
				Remote: RemoteCfg{
					AuthType:   "sshPassword",
					Host:       "bla.ble.com",
					Port:       "22",
					User:       "user",
					Password:   "bla",
					PrivateKey: "privKey",
					PassPhrase: "pass",
				},
				Dirs: []BackupDir{
					{
						Root: "relative/path",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
				},
				Mysql: []MysqlBackup{
					{
						DbName: "dbname",
						User:   "user",
						Pw:     "pw",
					},
				},
				Destination: "/backups",
				Keep:        3,
				Owner:       "ble",
				Mode:        "0700",
			},
		},

		{
			name: "profile with email notification",
			file: "sampledata/email.backup.yaml",
			want: Profile{
				Name: "email",

				Dirs: []BackupDir{
					{
						Root: "relative",
						Exclude: []glob.Glob{
							getGlob("*.log"),
						},
					},
				},
				Destination: "/backups",
				Keep:        3,
				Owner:       "ble",
				Mode:        "0700",
				Notify:      true,
				NotifyCfg: EmailNotify{
					Host:     "smtp.mail.com",
					Port:     "587",
					User:     "mail@mails.com",
					Password: "1234",
					To: []string{
						"mail1@mail.com",
						"mail2@mail.com",
					},
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

	//
	//t.Run("load nonexistent file", func(t *testing.T) {
	//	_, err := LoadProfileFile("sampledata/nonexistent.yaml")
	//	if err == nil {
	//		t.Fatal("expecting an error but none was returned")
	//	}
	//
	//	expectedErr := "open sampledata/nonexistent.yaml: no such file or directory"
	//	if err.Error() != expectedErr {
	//		t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
	//	}
	//})
	//
	//t.Run("load from wrong file type", func(t *testing.T) {
	//	_, err := LoadProfileFile("sampledata/json.json")
	//	if err == nil {
	//		t.Fatal("expecting an error but none was returned")
	//	}
	//
	//	expectedErr := "profile path is not a .yaml file"
	//	if err.Error() != expectedErr {
	//		t.Errorf("got unexpected error, \ngot: \n\"%s\" \nwant: \n\"%s\"", err.Error(), expectedErr)
	//	}
	//})

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
			"email",
			"test",
			"remote",
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
