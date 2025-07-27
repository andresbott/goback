package profile

import "github.com/gobwas/glob"

// local type to unmarshal
type backupDir struct {
	Root    string
	Exclude []string
}

type BackupDir struct {
	Root    string
	Exclude []glob.Glob
}

type MysqlBackup struct {
	DbName string
	User   string
	Pw     string
}

type AuthType string

const (
	RemotePassword   AuthType = "sshPassword"
	RemotePrivateKey AuthType = "sshKey"
	RemoteSshAgent   AuthType = "sshAgent"
)

type RemoteCfg struct {
	AuthType   AuthType `yaml:"type"`
	Host       string
	Port       string
	User       string
	Password   string
	PrivateKey string `yaml:"privateKey"`
	PassPhrase string `yaml:"passPhrase"`
}

type EmailNotify struct {
	Host     string
	Port     string
	User     string
	Password string
	To       []string
}

// local type to unmarshal
type profile struct {
	Name        string
	Remote      RemoteCfg
	Dirs        []backupDir
	Mysql       []MysqlBackup
	Destination string
	Keep        int
	Owner       string
	Mode        string
	Notify      EmailNotify
}

type Profile struct {
	Name        string
	IsRemote    bool
	Remote      RemoteCfg
	Dirs        []BackupDir
	Mysql       []MysqlBackup
	Destination string
	Keep        int
	Owner       string
	Mode        string
	Notify      bool
	NotifyCfg   EmailNotify
}
