// Package main provides the bd-examples CLI tool for running bash example scripts
// from the beads project as a testing/development sandbox.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/ui"
)

// Global flags
var (
	jsonOutput  bool
	verbose     bool
	forceColor  bool
	examplesDir string
)

// Output styling comes from internal/ui rather than a local palette.
//
// This file used to declare its own six lipgloss styles with the same Ayu hex
// values internal/ui already defines. The duplication went unnoticed until the
// 2026-08-09 upstream merge [co-gmlf3], when upstream moved to
// charm.land/lipgloss/v2 — which drops AdaptiveColor in favour of
// lipgloss.LightDark — and this file was the only thing in the tree still
// importing github.com/charmbracelet/lipgloss. That import is no longer in
// go.mod, so the package stopped building.
//
// internal/ui is the right home for it anyway: its init() probes for a dark
// background only when colour is actually enabled and honours NO_COLOR, neither
// of which the local copy did. Use ui.PassStyle, ui.WarnStyle, ui.FailStyle,
// ui.MutedStyle, ui.AccentStyle and ui.BoldStyle here.

var rootCmd = &cobra.Command{
	Use:   "bd-examples",
	Short: "Run beads bash examples as a testing sandbox",
	Long: `bd-examples is a CLI tool for running bash example scripts from the beads project.

It provides a safe sandbox environment for testing and development:
  - Dry-run mode by default (no state modifications)
  - Prerequisite checking before execution
  - Timestamped, colored output
  - Isolated sandbox creation for real testing

Examples:
  bd-examples list                    # List all available scripts
  bd-examples check bash-agent        # Check prerequisites
  bd-examples run bash-agent/agent.sh # Dry-run a script
  bd-examples sandbox --issues 10     # Create isolated test environment`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&forceColor, "color", false, "Force color output")
	rootCmd.PersistentFlags().StringVar(&examplesDir, "examples-dir", "", "Path to examples directory (auto-detected if not set)")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(sandboxCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.FailStyle.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

// findExamplesDir locates the examples directory
func findExamplesDir() (string, error) {
	if examplesDir != "" {
		if _, err := os.Stat(examplesDir); err != nil {
			return "", fmt.Errorf("specified examples directory not found: %s", examplesDir)
		}
		return examplesDir, nil
	}

	// Try current directory
	if _, err := os.Stat("examples"); err == nil {
		return "examples", nil
	}

	// Try relative to executable
	exe, err := os.Executable()
	if err == nil {
		dir := exe + "/../examples"
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("examples directory not found. Use --examples-dir to specify")
}
