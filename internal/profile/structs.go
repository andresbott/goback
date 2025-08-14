package profile

import (
	"github.com/gobwas/glob"
)

type Profile struct {
	Name string
	Type ProfileType

	Ssh  Ssh
	Dirs []BackupPath
	Dbs  []BackupDb

	Destination Destination
	Notify      EmailNotify
}

type ProfileType string

const (
	TypeRemote   ProfileType = "remote"
	TypeLocal    ProfileType = "local"
	TypeSftpSync ProfileType = "sftpsync"
)

// Ssh holds the details to connect over ssh to the remote
type Ssh struct {
	Type       ConnType
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string `yaml:"privateKey"`
	Passphrase string
}

type ConnType string

const (
	ConnTypePasswd   ConnType = "password"
	ConnTypeSshKey   ConnType = "sshkey"
	ConnTypeSshAgent ConnType = "sshagent"
)

// BackupPath Holds the details about a path to include in the backup
type BackupPath struct {
	Path    string
	Name    string // used only in sftp sync
	Exclude []glob.Glob
}

type BackupDb struct {
	Name          string
	Type          DbType
	ContainerName string `yaml:"containerName"`
	User          string
	Password      string
}
type DbType string

const (
	DbMysql          DbType = "mysql"
	DbDockerMysql    DbType = "dockermysql"
	DbMaria          DbType = "mariadb"
	DbDockerMaria    DbType = "dockermariadb"
	DbPostgres       DbType = "postgres"
	DbDockerPostgres DbType = "dockerpostgres"
)

type Destination struct {
	Path  string
	Keep  int
	Owner string
	Mode  string
}

type EmailNotify struct {
	Host      string
	Port      string
	User      string
	Password  string
	From      string
	To        []string
	OnSuccess bool `yaml:"onSuccess"`
}

// HasValues check if all the notification fields are of type default zero
func (m EmailNotify) HasValues() bool {
	if m.Host == "" ||
		m.Port == "" ||
		m.User == "" ||
		m.From == "" ||
		m.Password == "" {
		return false
	}
	if len(m.To) == 0 {
		return false
	}
	return true
}
