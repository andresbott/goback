package goback

import (
	"errors"
	"fmt"
	"git.andresbott.com/Golang/goback/lib/ssh"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.andresbott.com/Golang/goback/internal/profile"
)

// date string used to format the backup profiles
// resulting profiles will be <name>_2006_02_01-15:04:05_backup.zip
const dateStr = "2006_02_01-15:04:05"

//ExecuteSingleProfile runs all the profile actions on a single profile file
func ExecuteSingleProfile(flag string) error {
	prfl, err := profile.LoadProfile(flag)
	if err != nil {
		return err
	}

	err = BackupProfile(prfl)
	if err != nil {
		if prfl.Notify {
			// ignore notification error
			_ = NotifyFailure(prfl.NotifyCfg, err)
		}
		return err
	}

	// delete old backup files
	err = ExpurgeDir(prfl.Destination, prfl.Keep, prfl.Name)
	if err != nil {
		if prfl.Notify {
			// ignore notification error
			_ = NotifyFailure(prfl.NotifyCfg, err)
		}
		return err
	}
	if prfl.Notify {
		_ = NotifySuccess(prfl.NotifyCfg)
	}
	return nil
}

// ExecuteMultiProfile runs all the profile actions on all the backup profile files found in a directory
func ExecuteMultiProfile(flag string) error {
	profiles, err := profile.LoadProfiles(flag)
	if err != nil {
		return err
	}

	failedProfiles := []string{}
	failedDeletes := []string{}
	for _, prfl := range profiles {
		err = BackupProfile(prfl)
		if err != nil {
			if prfl.Notify {
				// ignore notification error
				_ = NotifyFailure(prfl.NotifyCfg, err)
			}
			failedProfiles = append(failedProfiles, prfl.Name+":"+err.Error())
		}

		// delete old backup files
		err = ExpurgeDir(prfl.Destination, prfl.Keep, prfl.Name)
		if err != nil {
			if prfl.Notify {
				// ignore notification error
				_ = NotifyFailure(prfl.NotifyCfg, err)
			}
			failedDeletes = append(failedDeletes, prfl.Name+":"+err.Error())
		}
		if prfl.Notify {
			_ = NotifySuccess(prfl.NotifyCfg)
		}
	}

	if len(failedProfiles) > 0 || len(failedDeletes) > 0 {
		msg := "errors while processing profiles:"
		if len(failedProfiles) > 0 {
			msg = msg + "backup profiles: " + strings.Join(failedProfiles, ",")
		}
		if len(failedDeletes) > 0 {
			msg = msg + "delete files: " + strings.Join(failedDeletes, ",")
		}
		return errors.New(msg)
	}
	return nil
}

// BackupProfile takes a single profile as input and generates a single Zip backup as output
func BackupProfile(prfl profile.Profile) error {

	// check if destination dir exists
	fInfo, err := os.Stat(prfl.Destination)
	if err != nil {
		return fmt.Errorf("destination not found:%v", err)
	}
	if !fInfo.IsDir() {
		return errors.New("the output path is not a directory")
	}

	dest := filepath.Join(prfl.Destination, getZipName(prfl.Name))

	if prfl.IsRemote {

		port, err := strconv.Atoi(prfl.Remote.Host)
		if err != nil {
			return fmt.Errorf("error parsisn port: %v", err)
		}

		sshC, err := ssh.New(ssh.Cfg{
			Host:          prfl.Remote.Host,
			Port:          port,
			Auth:          ssh.GetAuthType(prfl.Remote.RemoteType),
			User:          prfl.Remote.User,
			Password:      prfl.Remote.Password,
			PrivateKey:    prfl.Remote.PrivateKey,
			PassPhrase:    prfl.Remote.PassPhrase,
			IgnoreHostKey: false, // no need to expose that for the moment
		})

		if err != nil {
			return fmt.Errorf("error createing ssh client: %v", err)
		}
		err = sshC.Connect()
		if err != nil {
			return fmt.Errorf("error connecting ssh: %v", err)
		}
		defer func() {
			_ = sshC.Disconnect()
		}()

		switch prfl.Remote.RemoteType {
		case profile.SftpSync:
			err = SyncBackups(sshC, prfl.Remote.Path, dest, prfl.Name)
			if err != nil {
				return fmt.Errorf("error running sftp profile: %v", err)
			}
		case profile.SshAgent, profile.Password, profile.PrivateKey:
			err = backupSftp(sshC, prfl.Dirs, prfl.Mysql, dest)
			if err != nil {
				return fmt.Errorf("error running sftp profile: %v", err)
			}
		default:
			return fmt.Errorf("remote profile type %s is not valid", prfl.Remote.RemoteType)
		}

	} else {
		err := backupLocalFs(prfl.Dirs, prfl.Mysql, dest)
		if err != nil {
			return fmt.Errorf("error running local profile: %v", err)
		}
	}

	// change file ownership
	if prfl.Owner != "" {
		err := chown(dest, prfl.Owner)
		if err != nil {
			return fmt.Errorf("unable to change owner of file: \"%s\", %v", dest, err)
		}
	}

	// change file mode
	if prfl.Mode != "" {
		err := chmod(dest, prfl.Mode)
		if err != nil {
			return fmt.Errorf("unable to change perm of file: \"%s\", %v", dest, err)
		}
	}
	return nil
}

// deleteZipErr deletes the incomplete zip file in case onf an error, and returns the error
// if the delete opeation fails a new error is created that states both problems
func deleteZipErr(dest string, err error) error {
	//try to delete the zip file
	e := os.Remove(dest)
	if e != nil {
		return fmt.Errorf("unable to delete incomplete zip file due to: %v while handling error: %v", e, err)
	}
	return err
}

func chown(file string, owner string) error {
	usr, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("unable to find user: %s", err)
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return fmt.Errorf("user to id conversion: %v", err)
	}

	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return fmt.Errorf("user to gid conversion: %v", err)
	}

	err = os.Chown(file, uid, gid)
	if err != nil {
		return fmt.Errorf("chown failed: %v", err)
	}
	return nil
}

func chmod(file string, mode string) error {
	modeInt, err := strconv.Atoi(mode)
	if err != nil {
		return fmt.Errorf("type conversion: %v", err)
	}

	err = os.Chmod(file, os.FileMode(modeInt))
	if err != nil {
		return fmt.Errorf("chmod failed: %v", err)
	}
	return nil
}

// getZipName generates the name of the output zip based on the input and a date combinations
func getZipName(in string) string {
	dt := time.Now()
	return in + "_" + dt.Format(dateStr) + "_backup.zip"
}
