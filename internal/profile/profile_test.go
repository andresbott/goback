package profile

import (
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func getGlob(in string) glob.Glob {
	return glob.MustCompile(in)
}

var localProfile1 = Profile{
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
}

var localProfile2 = Profile{
	Name: "test2",
	Dirs: []BackupDir{
		{
			Root: "/bla2",
			Exclude: []glob.Glob{
				getGlob("*.log"),
			},
		},
		{
			Root: "/ble2",
			Exclude: []glob.Glob{
				getGlob("*.logs"),
			},
		},
	},
	Mysql: []MysqlBackup{
		{
			DbName: "dbname2",
			User:   "user",
			Pw:     "pw",
		},
	},
	Destination: "/backups",
	Keep:        3,
	Owner:       "ble",
	Mode:        "0700",
}

var remoteProfile = Profile{
	Name:     "remote",
	IsRemote: true,
	Remote: RemoteConnection{
		RemoteType: "sshPassword",
		Path:       "/backup/out",
		Host:       "bla.ble.com",
		Port:       "22",
		User:       "user",
		Password:   "bla",
		PrivateKey: "privKey",
		PassPhrase: "pass",
	},
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
}

var emailProfile = Profile{
	Name: "email",
	Dirs: []BackupDir{
		{
			Root: "relative",
			Exclude: []glob.Glob{
				getGlob("*.log"),
			},
		},
	},
	Notify: true,
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
	Destination: "/backups",
	Keep:        3,
	Owner:       "ble",
	Mode:        "0700",
}

func TestLoadProfile(t *testing.T) {

	t.Run("load existing file", func(t *testing.T) {
		got, err := LoadProfile("sampledata/local.backup.yaml")
		if err != nil {
			t.Fatal("umexpected error: ", err)
		}

		want := localProfile1
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("output mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("load remote profile", func(t *testing.T) {
		got, err := LoadProfile("sampledata/remote.backup.yaml")
		if err != nil {
			t.Fatal("umexpected error: ", err)
		}

		want := remoteProfile
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("output mismatch (-got +want):\n%s", diff)
		}
	})

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

func TestLoadProfiles(t *testing.T) {

	t.Run("load directory with profiles", func(t *testing.T) {
		got, err := LoadProfiles("sampledata")
		if err != nil {
			t.Fatal("umexpected error: ", err)
		}

		want := []Profile{
			emailProfile,
			localProfile1,
			localProfile2,
			remoteProfile,
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
