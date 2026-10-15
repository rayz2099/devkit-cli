package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func execute(args []string) error {
	cmd := newRoot()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "devkit-cli <list|clean|build|install> [module...]",
		Short:         "build and install devkit CLIs",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newList(), newTask("clean"), newTask("build"), newTask("install"))
	return cmd
}

func newList() *cobra.Command {
	return &cobra.Command{
		Use:   "list [module...]",
		Short: "list modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := selected(args)
			if err != nil {
				return err
			}
			for _, name := range names {
				fmt.Fprintln(cmd.OutOrStdout(), name.Name)
			}
			return nil
		},
	}
}

func newTask(task string) *cobra.Command {
	return &cobra.Command{
		Use:   task + " [module...]",
		Short: task + " modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			mods, err := selected(args)
			if err != nil {
				return err
			}
			root, err := findRoot()
			if err != nil {
				return err
			}
			dest := os.Getenv("DEST")
			if dest == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("home: %w", err)
				}
				dest = filepath.Join(home, ".local", "bin")
			}
			for _, mod := range mods {
				if err := runTask(root, dest, task, mod); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
