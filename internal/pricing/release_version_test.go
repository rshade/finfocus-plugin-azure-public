package pricing

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// releasePleaseExtraFile is one entry of a package's extra-files list. A plain
// string entry is a generic updater for that path.
type releasePleaseExtraFile struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	JSONPath string `json:"jsonpath"`
}

func (f *releasePleaseExtraFile) UnmarshalJSON(data []byte) error {
	var path string
	if err := json.Unmarshal(data, &path); err == nil {
		*f = releasePleaseExtraFile{Type: "generic", Path: path}
		return nil
	}
	type plain releasePleaseExtraFile
	return json.Unmarshal(data, (*plain)(f))
}

// TestReleasePleaseConfig_ExtraFiles_BumpPluginVersion guards issue #106: a
// release must bump pluginVersion and both manifests together, or the binary
// reports the previous version and the manifest byte comparison fails.
func TestReleasePleaseConfig_ExtraFiles_BumpPluginVersion(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../release-please-config.json")
	if err != nil {
		t.Fatalf("read release-please-config.json: %v", err)
	}
	var config struct {
		Packages map[string]struct {
			ExtraFiles []releasePleaseExtraFile `json:"extra-files"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("parse release-please-config.json: %v", err)
	}

	got := map[string]releasePleaseExtraFile{}
	for _, file := range config.Packages["."].ExtraFiles {
		got[file.Path] = file
	}
	for _, want := range []releasePleaseExtraFile{
		{Type: "generic", Path: "internal/pricing/calculator.go"},
		{Type: "json", Path: "manifest.json", JSONPath: "$.metadata.version"},
		{Type: "yaml", Path: "manifest.yaml", JSONPath: "$.metadata.version"},
	} {
		if got[want.Path] != want {
			t.Errorf("extra-files entry for %s = %+v, want %+v", want.Path, got[want.Path], want)
		}
	}

	source, err := os.ReadFile("calculator.go")
	if err != nil {
		t.Fatalf("read calculator.go: %v", err)
	}
	annotated := regexp.MustCompile(`(?m)^const pluginVersion = "[^"]+" // x-release-please-version$`)
	if !annotated.Match(source) {
		t.Error(`calculator.go: pluginVersion needs a trailing ` +
			`"// x-release-please-version" comment for the generic updater`)
	}
}
