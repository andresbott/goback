package rcp

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/davecgh/go-spew/spew"
	"io"
	"os"
	"sync"
)

func handleRemoteSession(in io.Reader, out io.WriteCloser, errOut io.Writer) {
	////scanner := bufio.NewScanner(in)
	//fmt.Fprintln(out, "Remote ready. Send file paths (type 'exit' to quit).")
	//fmt.Println("Remote ready. Send file paths (type 'exit' to quit).")

	defer out.Close()
	dec := gob.NewDecoder(in)
	prf := profile.Profile{}
	if err := dec.Decode(&prf); err != nil {
		fmt.Fprintf(errOut, "Error decoding profile: %v\n", err)
	}

	spew.Dump(prf)

	if prf.Name == "ERR" {
		fmt.Fprintf(errOut, "ERROR: reading Processing Profile")

		return
	}

	filePath := "/etc/hosts"
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(errOut, "failed to open file: %v \n", err)
		return
	}
	defer file.Close()

	// Copy file content to writer
	bufWriter := bufio.NewWriter(out)
	defer func() {
		bufWriter.Flush()
	}()

	_, err = io.Copy(bufWriter, file)
	if err != nil {
		fmt.Fprintf(errOut, "failed to copy data: %v \n", err)
		return
	}

}

func localClient(in io.Reader, out io.Writer, errOut io.Reader) error {
	// send the profile
	prf := profile.Profile{
		Name:        "ERR",
		IsRemote:    false,
		Remote:      profile.RemoteCfg{},
		Dirs:        nil,
		Mysql:       nil,
		Destination: "",
		Keep:        0,
		Owner:       "",
		Mode:        "0755",
		Notify:      true,
		NotifyCfg:   profile.EmailNotify{},
	}
	enc := gob.NewEncoder(out)

	err := enc.Encode(prf)
	if err != nil {
		panic(err)
	}

	// Use a WaitGroup to wait until `in` is fully read
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		io.Copy(os.Stdout, in) // Copy remote stdout to local stdout
	}()

	// Wait for remote output to finish
	wg.Wait()

	// handle errors
	remoteErr, err := io.ReadAll(errOut)
	if err != nil {
		panic(err)
	}
	if remoteErr != nil {
		return fmt.Errorf("operation failed: %s", string(remoteErr))
	}
	return nil

}

func Backend() {
	fmt.Println("Starting backend")

}
