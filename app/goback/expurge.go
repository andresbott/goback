package goback

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gobwas/glob"
)

// ExpurgeDir deletes all the older backups keeping N older versions of a specific backup profile name
func ExpurgeDir(path string, keepN int, name string) error {

	pathInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error getting stat of path: %v", err)
	}
	if !pathInfo.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	files, err := filepath.Glob(path + "/*.zip")
	if err != nil {
		return fmt.Errorf("error listing files in path: %v", err)
	}

	fileNames := []string{}
	for _, file := range files {
		fileNames = append(fileNames, filepath.Base(file))
	}

	filesToDelete, err := findToDelete(fileNames, name, keepN)
	if err != nil {
		return fmt.Errorf("error parsing files to delete: %v", err)
	}

	if len(filesToDelete) == 0 {
		return nil
	}

	for _, file := range filesToDelete {
		e := os.Remove(filepath.Join(path, file))
		if e != nil {
			return fmt.Errorf("unable to delete old zip file: %v", e)
		}
	}
	return nil
}

func findToDelete(files []string, profileName string, n int) ([]string, error) {

	if profileName == "" {
		return nil, errors.New("profile name cannot be empty")
	}

	// this glob patter matches any file with the pattern: name_2006_02_01-15:04:05_backup.zip
	pattern := profileName + "_[0-9][0-9][0-9][0-9]_[0-9][0-9]_[0-9][0-9]-[0-9][0-9]:[0-9][0-9]:[0-9][0-9]_backup.zip"
	g, _ := glob.Compile(pattern)

	found := []string{}
	for _, f := range files {
		if g.Match(f) {
			found = append(found, f)
		}
	}

	// early return if none match the desired condition
	if len(found) == 0 {
		return found, nil
	}

	// sort by date
	sort.SliceStable(found, func(i, j int) bool {
		dateI := extractTime(found[i])
		dateJ := extractTime(found[j])
		return dateI.Before(dateJ)
	})

	if len(found) <= n {
		n = len(found)
	}

	// drop the N newest (bottom of list) items from the list ( to not be deleted )
	return found[0 : len(found)-n], nil
}

// extractTime takes a filename and generates a time.Time for when the file was created
// since this is called after matching glob.Mach we are sure a valid date string is present and therefore ignore errors
func extractTime(in string) time.Time {

	// sample file pattern name_2006_02_01-15:04:05_backup.zip
	// this extracts the date-time string starting from the end of the file name
	parts := strings.Split(in, "_")
	parts = parts[len(parts)-4 : len(parts)-1]

	t, _ := time.Parse(dateStr, strings.Join(parts, "_"))
	return t

}
