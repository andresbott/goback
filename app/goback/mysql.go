package goback

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
)

func copyRemoteMysql(sshc *ssh.Client, binPath string, Db profile.BackupDb, zip *zip.Handler) (err error) {
	h, err := mysqldump.NewDocker(mysqldump.DockerCfg{
		BinPath: binPath,
		User:    Db.User,
		Pw:      Db.Password,
		DbName:  Db.Name,
	})
	if err != nil {
		return fmt.Errorf("nable to create mysqldump wrapper: %v", err)
	}

	sess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	outPipe, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to set stdout pipe, %v", err)
	}

	err = sess.Start(h.Cmd())
	if err != nil {
		return fmt.Errorf("unable to start ssh command: %v", err)
	}

	err = zip.WriteFile(outPipe, filepath.Join("_mysqldump", Db.Name+".dump.sql"))
	if err != nil {
		return fmt.Errorf("unable write mysqloutout to zip file, %v", err)
	}

	err = sess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %v", err)
	}

	return nil
}
