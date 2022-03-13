package profile

import (
	"errors"
	"fmt"
	"github.com/gobwas/glob"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// local type to unmarshal
type backupDir struct {
	Root           string
	FollowSymLinks bool
	Exclude        []string
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

type remoteConnection struct {
	RemoteType string `yaml:"type"`
	Url        string
	Path       string
	User       string
	Password   string
	PrivateKey string `yaml:"privateKey"`
	PassPhrase string `yaml:"passPhrase"`
}

type RemoteConnection struct {
	RemoteType string
	Host       string
	Port       string
	Path       string
	User       string
	Password   string
	PrivateKey string
	PassPhrase string
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
	Remote      remoteConnection
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
	Remote      RemoteConnection
	IsRemote    bool
	Dirs        []BackupDir
	Mysql       []MysqlBackup
	Destination string
	Keep        int
	Owner       string
	Mode        string
	Notify      bool
	NotifyCfg   EmailNotify
}

type RemoteType string

const (
	Password   = "sshPassword"
	PrivateKey = "sshKey"
	SshAgent   = "sshAgent"
	SftpSync   = "sftpSync"
)

// check if all the notification fields are of type default zero
func (m EmailNotify) isEmpty() bool {
	if m.Host != "" ||
		m.Port != "" ||
		m.User != "" ||
		m.Password != "" {
		return false
	}
	if m.To != nil && len(m.To) > 0 {
		return false
	}
	return true
}

// LoadProfile loads a profile from a yaml file
func LoadProfile(file string) (Profile, error) {

	fileExtension := filepath.Ext(file)
	if fileExtension != ".yaml" {
		return Profile{}, errors.New("profile path is not a .yaml file")
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return Profile{}, err
	}

	p := profile{}
	err = yaml.Unmarshal(data, &p)
	if err != nil {
		return Profile{}, err
	}

	ret := Profile{
		Name:        p.Name,
		Mysql:       p.Mysql,
		Destination: p.Destination,
		Keep:        p.Keep,
		Owner:       p.Owner,
		Mode:        p.Mode,
	}

	if ret.Name == "" {
		return Profile{}, errors.New("profile cannot be empty")
	}

	// check for remote connection settings
	if p.Remote.RemoteType != "" {
		ret.IsRemote = true

		t := p.Remote.RemoteType
		if t != "sshPassword" && t != "sshKey" && t != "sshAgent" {
			return Profile{}, fmt.Errorf("remote type %s is not allowed", t)
		}

		ret.Remote.RemoteType = p.Remote.RemoteType
		ret.Remote.Path = p.Remote.Path
		ret.Remote.User = p.Remote.User
		ret.Remote.Password = p.Remote.Password
		ret.Remote.PrivateKey = p.Remote.PrivateKey
		ret.Remote.PassPhrase = p.Remote.PassPhrase
		host, port, err := net.SplitHostPort(p.Remote.Url)

		if err != nil {
			if err.Error() == "address "+p.Remote.Url+": missing port in address" {
				port = getDefaultPort(p.Remote.RemoteType)
				host = p.Remote.Url
			} else {
				return Profile{}, fmt.Errorf("unable to parse url: %v", err)
			}
		}
		ret.Remote.Host = host
		ret.Remote.Port = port
	}

	// check notification config
	if !p.Notify.isEmpty() {
		ret.Notify = true
		ret.NotifyCfg = p.Notify
	}

	for _, bd := range p.Dirs {
		d := BackupDir{
			Root: bd.Root,
		}
		for _, excl := range bd.Exclude {
			g, gerr := glob.Compile(excl)
			if gerr != nil {
				return Profile{}, gerr
			}
			d.Exclude = append(d.Exclude, g)
		}
		ret.Dirs = append(ret.Dirs, d)
	}

	return ret, nil
}

func getDefaultPort(in string) string {
	switch in {
	case Password, PrivateKey, SshAgent, SftpSync:
		return "22"
	default:
		return ""
	}
}

const profileExt = ".backup.yaml"

func LoadProfiles(dir string) ([]Profile, error) {

	// check if dir exists
	finfo, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !finfo.IsDir() {
		return nil, errors.New("the path is not a directory")
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), profileExt) {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var profiles []Profile
	var errs []string

	for _, file := range files {
		p, perr := LoadProfile(file)
		if perr != nil {
			errs = append(errs, file)
			continue
		}
		profiles = append(profiles, p)
	}

	if len(errs) > 0 {
		err = errors.New("errors loading profile from files: " + strings.Join(errs, ","))
	}

	return profiles, err
}

func Boilerplate() string {
	s := `---
# profile name used to identify different backup profiles
name: myService

# if goback should run on a remote server instead of locally, leave empty to run locally
remote:
    # the type of connection, currently valid: sshPassword | sshKey | sshAgent | sftpSync
    # if the type is set to sshAgent, get the ssh key from a running ssh agent
    type: sshPassword
    url: bla.ble.com:22
	# if type is sftpSync then this is the location where goback will sync from
	path: /var/goback
	# the username used to login to the server
    user: user
    # password used when type is sshPassword 
    password: bla
	# key file used when type is sshKey, a passphrase can be provided as well
    privateKey: privKey
    passPhrase: pass

# list of different filesystem directories to backup
dirs:
    # the rood path to use as backup
  - root: /home/user/
    # list of glop patterns of files to be excluded from this group
    exclude:
      - "*.log"

# list of mysql databases to be added to the profile
mysql:
    # database name is required, user and password will be used if provided
    # otherwise the tool will try to get them from common system locations, e.g. /etc/my.cnf
  - dbname: dbname
    user: user
    pw: pw

# this is the destination where the backup file will be written
# only local filesystem is allowed
destination: /backups

# how many older backups to keep for this profile
keep: 3

#change owner/mode of the generated file
owner : "ble"
mode : "0700"

# notify per email if a profile was successfull or not
notify:
	# send email to these addresses 
    to:
      - mail1@mail.com
      - mail2@mail.com

	# email server details
    host: smtp.mail.com
    port: 587
    user: mail@mails.com
    password: 1234

`
	return s
}
