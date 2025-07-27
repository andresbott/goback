package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"io"
	"path/filepath"
)

// dump a single database into a zip handler
func copyLocalMysql(binPath string, dbPrfl profile.MysqlBackup, zipHandler *zip.Handler) error {

	dbHandler, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
	})

	if err != nil {
		return err
	}

	zipWriter, err := zipHandler.FileWriter(filepath.Join("_mysqldump", dbPrfl.DbName+".dump.sql"))
	if err != nil {
		return err
	}
	err = dbHandler.Exec(zipWriter)
	if err != nil {
		return err
	}
	return nil
}

func copyRemoteMysql(sshc *ssh.Client, binPath string, dbPrfl profile.MysqlBackup, zip *zip.Handler) (err error) {
	h, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
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

	err = zip.WriteFile(outPipe, filepath.Join("_mysqldump", dbPrfl.DbName+".dump.sql"))
	if err != nil {
		return fmt.Errorf("unable write mysqloutout to zip file, %v", err)
	}

	err = sess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %v", err)
	}

	return nil
}
