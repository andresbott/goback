package cmd

import (
	"fmt"
	"github.com/AndresBott/goback/app/goback"
	"github.com/AndresBott/goback/app/metainfo"
	"github.com/AndresBott/goback/internal/clilog"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"runtime"
)

// Execute is the entry point for the command line
func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goback",
		Short: "goback is a simple backup tool",
		Long:  "goback is an opinionated tool to backup data from different sources like file system directories, mysql databases",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		_ = cmd.Help()
		return nil
	}

	cmd.Flags().SortFlags = false

	cmd.SilenceUsage = true
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		_ = cmd.Help()
		return nil
	})

	cmd.AddCommand(
		versionCmd(),
		generateCmd(),
		backupCmd(),
	)

	return cmd
}

func backupCmd() *cobra.Command {

	quiet := false
	cmd := cobra.Command{
		Use:   "backup",
		Short: "backup a profile or a directory",
		Long:  `backup a profile or a directory`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			printer := clilog.CliOut{
				Quiet:  quiet,
				Prefix: "-> ",
			}

			file := args[0]

			absPath, err := filepath.Abs(file)
			if err != nil {
				return err
			}

			fstat, err := os.Stat(absPath)
			if err != nil {
				return err
			}
			if fstat.IsDir() {
				return backupFromDir(absPath, &printer)
			} else {
				return backupFromFile(absPath, &printer)

			}
		},
	}

	// hide persistent flag on this command
	cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("pers")
		command.Parent().HelpFunc()(command, strings)
	})
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "do not print output")

	return &cmd
}

func backupFromFile(absFile string, printer *clilog.CliOut) error {
	printer.Print(fmt.Sprintf("using file %s", absFile))
	runner := goback.BackupRunner{
		Printer: printer,
	}

	err := runner.LoadProfileFile(absFile)
	if err != nil {
		return err
	}

	err = runner.Run()
	if err != nil {
		return err
	}

	return nil
}

func backupFromDir(absPath string, printer *clilog.CliOut) error {
	printer.Print(fmt.Sprintf("using Dir %s", absPath))
	// handle a directory containing profiles
	runner := goback.BackupRunner{
		Printer: printer,
	}
	err := runner.LoadProfilesDir(absPath)
	if err != nil {
		return err
	}

	err = runner.Run()
	if err != nil {
		return err
	}

	return nil
}

func generateCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "generate",
		Short: "generate a profile file",
		Long:  `generate a profile file`,
		Run: func(cmd *cobra.Command, args []string) {
			// we just print the profile boilerplate to stdout
			fmt.Println(profile.Boilerplate())
		},
	}

	// hide persistent flag on this command
	cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("pers")
		command.Parent().HelpFunc()(command, strings)
	})

	return &cmd
}

func versionCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "version ",
		Long:  `version long`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\n", metainfo.Version)
			fmt.Printf("Build date: %s\n", metainfo.BuildTime)
			fmt.Printf("Commit sha: %s\n", metainfo.ShaVer)
			fmt.Printf("Compiler: %s\n", runtime.Version())
		},
	}

	// hide persistent flag on this command
	cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("pers")
		command.Parent().HelpFunc()(command, strings)
	})

	return &cmd
}
