package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/config"
	"github.com/zigamedved/dotenv-doctor/internal/discover"
)

// resolveProjects centralizes scan-path resolution and discovery so every
// command behaves the same way:
//
//  1. --path flags override everything (no config touched)
//  2. config file scan_paths if config exists
//  3. else smart-walk from $PWD (git root if any, else CWD)
//
// First-run logic (prompting the user to set up config) lives in `envs init`
// and is invoked from main when the user runs `envs` with no args and no config.
func resolveProjects(cmd *cobra.Command) ([]discover.Project, []string, error) {
	overridePaths, _ := cmd.Flags().GetStringSlice("path")

	var paths []string
	switch {
	case len(overridePaths) > 0:
		paths = overridePaths
	case config.Exists():
		c, err := config.Load()
		if err != nil {
			return nil, nil, fmt.Errorf("load config: %w", err)
		}
		paths = c.ScanPaths
	default:
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		paths = discover.SmartWalk(cwd)
	}

	projects, err := discover.Discover(discover.Options{ScanPaths: paths})
	if err != nil {
		return nil, nil, fmt.Errorf("discover: %w", err)
	}
	return projects, paths, nil
}
