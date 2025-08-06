package profile

import (
	"errors"
	"fmt"
	"github.com/gobwas/glob"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// unmarshal the yaml into small struct to get the version of the config file
func getVersion(data []byte) (int, error) {
	type vStruct struct {
		Version int
	}
	d := &vStruct{}
	err := yaml.Unmarshal(data, &d)
	if err != nil {
		return 0, err
	}
	return d.Version, nil
}

// LoadProfile loads a profile from a yaml file
func LoadProfile(file string) (Profile, error) {

	fileExtension := filepath.Ext(file)
	if fileExtension != ".yaml" {
		return Profile{}, errors.New("profile path is not a .yaml file")
	}

	// #nosec G304 -- file path is only unmarshalled into yaml
	data, err := os.ReadFile(file)
	if err != nil {
		return Profile{}, err
	}

	version, err := getVersion(data)
	if err != nil {
		return Profile{}, fmt.Errorf("unable to get profile version: %w", err)
	}

	switch version {
	case 1:
		return loadProfileV1(data)
	default:
		return Profile{}, fmt.Errorf("unsupported profile version: %d", version)
	}
}

type profileV1 struct {
	Name string
	Type ProfileType

	Ssh  Ssh
	Dirs []struct {
		Path    string
		Name    string
		Exclude []string
	}
	Dbs []BackupDb

	Destination Destination
	Notify      EmailNotify
}

// load Profile V1 and return a valid profile
func loadProfileV1(data []byte) (Profile, error) {
	loadedProfile := profileV1{}
	err := yaml.Unmarshal(data, &loadedProfile)
	if err != nil {
		return Profile{}, err
	}

	if loadedProfile.Type == "" {
		return Profile{}, errors.New("profile has no type")
	}

	returnProfile := Profile{
		Name:        loadedProfile.Name,
		Type:        ProfileType(strings.ToLower(string(loadedProfile.Type))),
		Ssh:         loadedProfile.Ssh,
		Destination: loadedProfile.Destination,
		Notify:      loadedProfile.Notify,
	}

	if !slices.Contains([]ProfileType{TypeSftpSync, TypeLocal, TypeRemote}, returnProfile.Type) {
		return Profile{}, errors.New("profile has invalid type")
	}

	// ensure values are lower type
	returnProfile.Ssh.Type = ConnType(strings.ToLower(string(loadedProfile.Ssh.Type)))

	if returnProfile.Name == "" {
		return Profile{}, errors.New("profile name cannot be empty")
	}

	// requires ssh config
	if slices.Contains([]ProfileType{TypeSftpSync, TypeRemote}, returnProfile.Type) {
		if !slices.Contains([]ConnType{ConnTypeSshKey, ConnTypePasswd, ConnTypeSshAgent}, returnProfile.Ssh.Type) || returnProfile.Ssh.Type == "" {
			return Profile{}, errors.New("profile has invalid ssh connection type")
		}
		if returnProfile.Ssh.Host == "" {
			return Profile{}, errors.New("profile ssh host cannot be empty")
		}

		if returnProfile.Ssh.Port == 0 {
			returnProfile.Ssh.Port = 22
		}
	}

	// requires bbackup targets
	if slices.Contains([]ProfileType{TypeLocal, TypeRemote}, returnProfile.Type) {
		if len(loadedProfile.Dbs) == 0 && len(loadedProfile.Dirs) == 0 {
			return Profile{}, errors.New("nothing to backup")
		}
	}

	// handle directories
	for _, dir := range loadedProfile.Dirs {
		d := BackupPath{
			Path: dir.Path,
			Name: dir.Name,
		}
		for _, excl := range dir.Exclude {
			g, gerr := glob.Compile(excl)
			if gerr != nil {
				return Profile{}, fmt.Errorf("unable to compile exclude pattern: %w", gerr)
			}
			d.Exclude = append(d.Exclude, g)
		}
		if d.Path == "" {
			return Profile{}, errors.New("profile path cannot be empty")
		}

		// if type is sftpsync we also require a profile name
		if returnProfile.Type == TypeSftpSync && d.Name == "" {
			return Profile{}, errors.New("profile name for sync path cannot be empty")
		}

		returnProfile.Dirs = append(returnProfile.Dirs, d)
	}

	// Handle DBs
	for _, db := range loadedProfile.Dbs {
		d := BackupDb{
			Name:     db.Name,
			Type:     DbType(strings.ToLower(string(db.Type))),
			User:     db.User,
			Password: db.Password,
		}
		returnProfile.Dbs = append(returnProfile.Dbs, d)
	}
	return returnProfile, nil
}

const profileExt = ".backup.yaml"

// LoadProfiles will try to load all profiles in a directory, no error is returned if all profiles are ok
// if any profile is incomplete, the slice of profiles still contain the valid profiles
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
	var errs error
	for _, file := range files {
		p, perr := LoadProfile(file)
		if perr != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to load profile %s: %w", file, perr))
			continue
		}
		profiles = append(profiles, p)
	}

	return profiles, errs
}

// todo replace with embedd
func Boilerplate() string {
	s := `---
# make sure your filename end with .backup.yaml to be picked up.

# profile name used to identify different backup profiles
name: myService

# use a remote connection to run goback
remote:
	# the type of connection, currently valid: sshPassword | sshKey | sshAgent 
	# if the type is set to sshAgent, get the ssh key from a running ssh agent
	type: sshPassword
	#host/port of the server
	host: bla.ble.com
	port: 22
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
# this also affects the output of a synced directory
keep: 3

#change owner/mode of the generated file
owner : "ble"
mode : "0700"

# notify per email if a profile was successful or not
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
