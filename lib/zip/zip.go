package zip

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type Handler struct {
	isOpen    bool
	file      *os.File
	zipWriter *zip.Writer
}

// Close the zipwriter as well as the file handler
func (z *Handler) Close() {
	if z.isOpen {
		z.isOpen = false
		_ = z.zipWriter.Close()
		_ = z.file.Close()
	}
}

// New creates a handler that holds the reference to the zip writer as well as the
// underlying file, the close method needs to be called at the end of using it
func New(dest string) (*Handler, error) {

	if !strings.HasSuffix(dest, ".zip") {
		return nil, errors.New("destination does not end in .zip")
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	file, err := os.OpenFile(dest, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip for writing: %s", err)
	}

	zh := Handler{
		isOpen:    true,
		file:      file,
		zipWriter: zip.NewWriter(file),
	}
	return &zh, nil
}

// AddFile writes a file into the current zip file
func (z *Handler) AddFile(origin string, zipDest string) error {
	if !z.isOpen {
		return errors.New("zip handler is closed")
	}

	file, err := os.Open(origin)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", origin, err)
	}
	defer file.Close()

	return z.WriteFile(file, zipDest)
}

// AddSymlink writes a symlink into a zip file
func (z *Handler) AddSymlink(origin string, zipDest string) error {
	file, err := os.Open(origin)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", origin, err)
	}
	defer file.Close()

	info, err := os.Lstat(file.Name())
	if err != nil {
		return fmt.Errorf("failed to stat link: %s", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to extract header from link: %s", err)
	}
	header.Method = zip.Deflate
	header.Name = zipDest

	writer, err := z.zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create header from link: %s", err)
	}

	linkVal, err := os.Readlink(origin)
	if err != nil {
		return fmt.Errorf("failed to read target from link: %s", err)
	}

	// Write symlink's target to writer - file's body for symlinks is the symlink target.
	_, err = writer.Write([]byte(linkVal))
	if err != nil {
		return fmt.Errorf("failed to write link into zip file: %s", err)
	}
	return nil
}

// WriteFile reads from a reader and copies the bytes over into a zip file
func (z *Handler) WriteFile(in io.Reader, dest string) error {

	wr, err := z.FileWriter(dest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(wr, in); err != nil {
		return fmt.Errorf("failed to write from reader to zip: %s", err)
	}

	return nil
}

// FileWriter returns a io.writer used to write to the file within the zip file defined with dest
func (z Handler) FileWriter(dest string) (io.Writer, error) {
	if !z.isOpen {
		return nil, errors.New("zip handler is closed")
	}

	wr, err := z.zipWriter.Create(dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create entry for %s in zip file: %s", dest, err)
	}
	return wr, nil
}

func ListFiles(in string) ([]string, error) {
	read, err := zip.OpenReader(in)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %s", err)
	}
	defer read.Close()

	files := []string{}
	for _, file := range read.File {
		files = append(files, file.Name)
	}
	return files, nil
}
