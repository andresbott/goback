package goback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// dump a single database into a zip handler
func copyLocalMysql(binPath string, Db profile.BackupDb, zipHandler *zip.Handler) error {

	dbHandler, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    Db.User,
		Pw:      Db.Password,
		DbName:  Db.Name,
	})

	if err != nil {
		return err
	}

	zipWriter, err := zipHandler.FileWriter(filepath.Join("_mysqldump", Db.Name+".dump.sql"))
	if err != nil {
		return err
	}
	err = dbHandler.Exec(zipWriter)
	if err != nil {
		return err
	}
	return nil
}

// dump a single database (that is running in a docker container) into a zip handler
func copyLocalDockerMysql(containerName string, Db profile.BackupDb, zipHandler *zip.Handler) error {
	ctx := context.Background()

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("unable to create docker client: %v", err)
	}
	defer dockerClient.Close()

	// Execute mysqldump inside the container
	var args []string
	if Db.User != "" {
		args = append(args, "-u", Db.User)
	}
	if Db.Password != "" {
		args = append(args, "-p"+Db.Password)
	}
	args = append(args,
		"--add-drop-database",
		"--databases",
		Db.Name,
	)

	// Create the exec
	execResp, err := dockerClient.ContainerExecCreate(ctx, containerName, types.ExecConfig{
		Cmd:          append([]string{"/usr/local/bin/mysqldump"}, args...),
		AttachStdout: true,
		AttachStderr: false,
	})
	if err != nil {
		return fmt.Errorf("unable to create container exec: %v", err)
	}

	// Attach to the exec to get the output
	output, err := dockerClient.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("unable to attach to container exec: %v", err)
	}
	defer output.Close()

	// Write the output to the zip file
	zipWriter, err := zipHandler.FileWriter(filepath.Join("_mysqldump", Db.Name+".dump.sql"))
	if err != nil {
		return fmt.Errorf("unable to create zip writer: %v", err)
	}

	// Copy the output, skipping the first 8 bytes which are the Docker protocol header
	header := make([]byte, 8)
	_, err = output.Reader.Read(header)
	if err != nil {
		return fmt.Errorf("unable to read Docker protocol header: %v", err)
	}

	_, err = io.Copy(zipWriter, output.Reader)
	if err != nil {
		return fmt.Errorf("unable to copy mysqldump output to zip: %v", err)
	}

	// Wait for the exec to complete and get the exit code
	inspectResp, err := dockerClient.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return fmt.Errorf("unable to inspect container exec: %v", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("mysqldump failed with exit code %d", inspectResp.ExitCode)
	}

	return nil
}

func copyRemoteMysql(sshc *ssh.Client, binPath string, Db profile.BackupDb, zip *zip.Handler) (err error) {
	h, err := mysqldump.New(mysqldump.Cfg{
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
