package profile

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gobwas/glob"
	"gopkg.in/yaml.v2"
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

	returnProfile, err := createProfile(loadedProfile)
	if err != nil {
		return Profile{}, err
	}

	if err := validateSshConfig(&returnProfile); err != nil {
		return Profile{}, err
	}

	if err := validateBackupTargets(loadedProfile, returnProfile.Type); err != nil {
		return Profile{}, err
	}

	dirs, err := processDirectories(loadedProfile.Dirs, returnProfile.Type)
	if err != nil {
		return Profile{}, err
	}
	returnProfile.Dirs = dirs

	dbs, err := processDatabases(loadedProfile.Dbs)
	if err != nil {
		return Profile{}, err
	}
	returnProfile.Dbs = dbs

	return returnProfile, nil
}

// createProfile creates and validates the basic profile structure
func createProfile(loadedProfile profileV1) (Profile, error) {
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

	// ensure values are lower case
	returnProfile.Ssh.Type = ConnType(strings.ToLower(string(loadedProfile.Ssh.Type)))

	if returnProfile.Name == "" {
		return Profile{}, errors.New("profile name cannot be empty")
	}

	return returnProfile, nil
}

// validateSshConfig validates SSH configuration for profiles that require it
func validateSshConfig(profile *Profile) error {
	// requires ssh config
	if slices.Contains([]ProfileType{TypeSftpSync, TypeRemote}, profile.Type) {
		if !slices.Contains([]ConnType{ConnTypeSshKey, ConnTypePasswd, ConnTypeSshAgent}, profile.Ssh.Type) || profile.Ssh.Type == "" {
			return errors.New("profile has invalid ssh connection type")
		}
		if profile.Ssh.Host == "" {
			return errors.New("profile ssh host cannot be empty")
		}

		if profile.Ssh.Port == 0 {
			profile.Ssh.Port = 22
		}
	}
	return nil
}

// validateBackupTargets ensures profiles that need backup targets have them
func validateBackupTargets(loadedProfile profileV1, profileType ProfileType) error {
	if slices.Contains([]ProfileType{TypeLocal, TypeRemote}, profileType) {
		if len(loadedProfile.Dbs) == 0 && len(loadedProfile.Dirs) == 0 {
			return errors.New("nothing to backup")
		}
	}
	return nil
}

// processDirectories processes and validates directory configurations
func processDirectories(dirs []struct {
	Path    string
	Name    string
	Exclude []string
}, profileType ProfileType) ([]BackupPath, error) {
	var backupDirs []BackupPath

	for _, dir := range dirs {
		d := BackupPath{
			Path: dir.Path,
			Name: dir.Name,
		}

		for _, excl := range dir.Exclude {
			g, gerr := glob.Compile(excl)
			if gerr != nil {
				return nil, fmt.Errorf("unable to compile exclude pattern: %w", gerr)
			}
			d.Exclude = append(d.Exclude, g)
		}

		if d.Path == "" {
			return nil, errors.New("profile path cannot be empty")
		}

		// if type is sftpsync we also require a profile name
		if profileType == TypeSftpSync && d.Name == "" {
			return nil, errors.New("profile name for sync path cannot be empty")
		}

		backupDirs = append(backupDirs, d)
	}

	return backupDirs, nil
}

// processDatabases processes and validates database configurations
func processDatabases(dbs []BackupDb) ([]BackupDb, error) {
	var backupDbs []BackupDb

	for _, db := range dbs {
		d := BackupDb{
			Name:          db.Name,
			Type:          DbType(strings.ToLower(string(db.Type))),
			User:          db.User,
			Password:      db.Password,
			ContainerName: db.ContainerName,
		}

		if slices.Contains([]DbType{DbDockerPostgres, DbDockerMysql}, d.Type) {
			if d.ContainerName == "" {
				return nil, errors.New("DB container name cannot be empty")
			}
		}

		backupDbs = append(backupDbs, d)
	}

	return backupDbs, nil
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

//go:embed configv1.yaml
var configV1Yaml string

func ConfigV1() string {
	return configV1Yaml
}
