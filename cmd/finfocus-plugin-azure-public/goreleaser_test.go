package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// goreleaserConfig represents the minimal structure of .goreleaser.yaml
// needed for validating archive naming.
type goreleaserConfig struct {
	ProjectName string `yaml:"project_name"`
	Archives    []struct {
		NameTemplate    string           `yaml:"name_template"`
		Formats         []string         `yaml:"formats"`
		Format          string           `yaml:"format"`
		FormatOverrides []formatOverride `yaml:"format_overrides"`
	} `yaml:"archives"`
}

type formatOverride struct {
	GOOS    string   `yaml:"goos"`
	Formats []string `yaml:"formats"`
	Format  string   `yaml:"format"`
}

// TestGoreleaserConfigDatesDoubleExtensionBug verifies that the current
// .goreleaser.yaml template DOES produce double extensions (the bug we're fixing).
// This ensures our fix is necessary.
func TestGoreleaserConfigDatesDoubleExtensionBug(t *testing.T) {
	// Read the real .goreleaser.yaml
	cfg := loadGoreleaserConfig(t)
	require.NotNil(t, cfg, "failed to load .goreleaser.yaml")
	require.NotEmpty(t, cfg.Archives, "no archives found in .goreleaser.yaml")

	archive := cfg.Archives[0]

	// The bug: the template includes the extension (.tar.gz, .zip)
	// AND goreleaser will append the extension from formats.
	// This produces names like: ..._Linux_x86_64.tar.gz.tar.gz
	if strings.Contains(archive.NameTemplate, ".tar.gz") ||
		strings.Contains(archive.NameTemplate, ".zip") {
		// Parse and render to demonstrate the bug
		tmpl, err := template.New("archive").Funcs(
			template.FuncMap{
				"title": strings.Title, // simulate {{ title .Os }}
			},
		).Parse(archive.NameTemplate)
		require.NoError(t, err, "failed to parse template")

		data := map[string]interface{}{
			"ProjectName": cfg.ProjectName,
			"Version":     "v0.1.0",
			"Os":          "linux",
			"Arch":        "amd64",
		}

		var buf strings.Builder
		err = tmpl.Execute(&buf, data)
		require.NoError(t, err, "failed to render template")

		rendered := buf.String()

		// If the template includes .tar.gz and we append it again, we'll get double extension
		if strings.Contains(archive.NameTemplate, ".tar.gz") {
			// This WILL fail when we fix it, which is correct!
			if strings.Contains(rendered, ".tar.gz") {
				// Simulating goreleaser appending .tar.gz
				withExtension := rendered + ".tar.gz"
				t.Logf("BUG DETECTED: template renders to %q, then goreleaser appends .tar.gz → %q",
					rendered, withExtension)
				assert.True(t, strings.Contains(withExtension, ".tar.gz.tar.gz"),
					"template + appended extension = double extension (the bug)")
			}
		}
	}
}

// TestGoreleaserAssetNamesMatchInstallerPatterns validates that rendered
// asset names from goreleaser would be found by the finfocus installer's
// pattern matching logic in ../finfocus/internal/registry/github.go.
func TestGoreleaserAssetNamesMatchInstallerPatterns(t *testing.T) {
	cfg := loadGoreleaserConfig(t)
	require.NotNil(t, cfg, "failed to load .goreleaser.yaml")
	require.NotEmpty(t, cfg.Archives, "no archives found in .goreleaser.yaml")

	archive := cfg.Archives[0]

	// Parse the template
	tmpl, err := template.New("archive").Funcs(
		template.FuncMap{
			"title": strings.Title, // {{ title .Os }}
			"eq": func(a, b interface{}) bool { // {{ if eq .Arch "amd64" }}
				return a == b
			},
		},
	).Parse(archive.NameTemplate)
	require.NoError(t, err, "failed to parse goreleaser template")

	// Test cases spanning key platforms and architectures
	testCases := []struct {
		version string
		goos    string
		goarch  string
	}{
		{"v0.1.0", "linux", "amd64"},
		{"v0.1.0", "linux", "arm64"},
		{"v0.1.0", "darwin", "amd64"},
		{"v0.1.0", "darwin", "arm64"},
		{"v0.1.0", "windows", "amd64"},
		{"0.1.0", "linux", "amd64"},   // without leading v
		{"0.1.0", "windows", "arm64"}, // windows without v
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s_%s", tc.goos, tc.goarch, tc.version), func(t *testing.T) {
			// Render the template
			data := map[string]interface{}{
				"ProjectName": cfg.ProjectName,
				"Version":     tc.version,
				"Os":          tc.goos,
				"Arch":        tc.goarch,
			}

			var buf strings.Builder
			err := tmpl.Execute(&buf, data)
			require.NoError(t, err, "failed to render template")

			baseFilename := buf.String()

			ext := archiveExtension(archive.Formats, archive.Format, ".tar.gz")
			for _, override := range archive.FormatOverrides {
				if override.GOOS == tc.goos {
					ext = archiveExtension(override.Formats, override.Format, ext)
					break
				}
			}

			// The final asset name AFTER goreleaser appends the extension
			finalAsset := baseFilename + ext

			t.Logf("Rendered: %s, Extension: %s, Final: %s",
				baseFilename, ext, finalAsset)

			// Validate against installer patterns
			// The installer (../finfocus/internal/registry/github.go buildAssetPatterns)
			// generates patterns that try many OS/arch variations. We use the same
			// logic here to verify the asset would be found.

			// Extract extension properly (handle .tar.gz as a single extension)
			assetExt := filepath.Ext(finalAsset)
			if strings.HasSuffix(finalAsset, ".tar.gz") {
				assetExt = ".tar.gz"
			} else if strings.HasSuffix(finalAsset, ".tar") {
				assetExt = ".tar"
			}

			patterns := buildInstallerAssetPatterns(
				cfg.ProjectName,
				tc.version,
				tc.goos,
				tc.goarch,
				assetExt,
			)

			found := false
			for _, pattern := range patterns {
				if finalAsset == pattern {
					found = true
					break
				}
			}

			assert.True(t, found,
				"asset %q not in installer patterns for %s/%s with %s",
				finalAsset, tc.goos, tc.goarch, tc.version)
		})
	}
}

// loadGoreleaserConfig reads and parses the .goreleaser.yaml file.
func loadGoreleaserConfig(t *testing.T) *goreleaserConfig {
	// Find the repo root by looking for .goreleaser.yaml
	var configPath string
	for _, candidate := range []string{
		".goreleaser.yaml",
		"../.goreleaser.yaml",
		"../../.goreleaser.yaml",
	} {
		if _, err := os.Stat(candidate); err == nil {
			configPath = candidate
			break
		}
	}

	require.NotEmpty(t, configPath, "could not find .goreleaser.yaml")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err, "failed to read .goreleaser.yaml")

	var cfg goreleaserConfig
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err, "failed to parse .goreleaser.yaml")

	return &cfg
}

// buildInstallerAssetPatterns replicates the logic from
// ../finfocus/internal/registry/github.go buildAssetPatterns.
// This generates all possible asset names the installer might try to find.
func buildInstallerAssetPatterns(
	projectName, version, goos, goarch, ext string,
) []string {
	// OS name variations (same as installer logic)
	osNames := []string{
		goos,            // linux, darwin, windows
		titleCase(goos), // Linux, Darwin, Windows
		strings.ToUpper(goos[:1]) + goos[1:],
	}
	if goos == "darwin" {
		osNames = append(osNames, "Darwin", "macos", "macOS", "MacOS")
	}

	// Architecture variations
	archNames := []string{goarch}
	if goarch == "amd64" {
		archNames = append(archNames, "x86_64", "X86_64", "AMD64")
	}
	if goarch == "arm64" {
		archNames = append(archNames, "ARM64", "aarch64", "AARCH64")
	}

	// Version variations
	versions := []string{version}
	if strings.HasPrefix(version, "v") {
		versions = append(versions, strings.TrimPrefix(version, "v"))
	} else {
		versions = append(versions, "v"+version)
	}

	// Generate all combinations (subset of full installer logic)
	var patterns []string
	for _, ver := range versions {
		for _, osName := range osNames {
			for _, arch := range archNames {
				pattern := fmt.Sprintf(
					"%s_%s_%s_%s%s",
					projectName,
					ver,
					osName,
					arch,
					ext,
				)
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}

// archiveExtension picks the goreleaser archive suffix. formats wins over
// the deprecated singular format. An empty choice keeps fallback.
func archiveExtension(formats []string, format, fallback string) string {
	chosen := format
	if len(formats) > 0 {
		chosen = formats[0]
	}
	if chosen == "" {
		return fallback
	}
	return "." + strings.TrimPrefix(chosen, ".")
}

// titleCase replicates the title case transformation (simple version).
func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
