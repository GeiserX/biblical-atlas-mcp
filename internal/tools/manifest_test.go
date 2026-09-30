package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestManifestVersions keeps package.json and server.json on one version.
func TestManifestVersions(t *testing.T) {
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		MCPName string `json:"mcpName"`
		License string `json:"license"`
		Bin     map[string]string
		Files   []string
	}
	var srv struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Packages    []struct {
			RegistryType string `json:"registryType"`
			Identifier   string `json:"identifier"`
			Version      string `json:"version"`
		} `json:"packages"`
	}
	readJSON(t, "../../package.json", &pkg)
	readJSON(t, "../../server.json", &srv)
	if pkg.Version == "" || pkg.Version != srv.Version {
		t.Fatalf("package.json %q and server.json %q disagree", pkg.Version, srv.Version)
	}
	if len(srv.Packages) != 1 || srv.Packages[0].RegistryType != "npm" || srv.Packages[0].Version != pkg.Version || srv.Packages[0].Identifier != pkg.Name {
		t.Fatalf("server.json packages: %+v", srv.Packages)
	}
	if pkg.MCPName != srv.Name {
		t.Fatalf("mcpName %q, server.json name %q", pkg.MCPName, srv.Name)
	}
	if len(srv.Description) >= 100 {
		t.Fatalf("server.json description is %d characters; the registry wants under 100", len(srv.Description))
	}
	if pkg.License != "MIT" {
		t.Fatalf("license %q", pkg.License)
	}
	if pkg.Bin["biblical-atlas-mcp"] != "run.js" {
		t.Fatalf("bin %v: npm strips a leading ./", pkg.Bin)
	}
}

// TestArchiveNames checks that postinstall.js asks for the archives
// GoReleaser builds.
func TestArchiveNames(t *testing.T) {
	gr := readFile(t, "../../.goreleaser.yaml")
	js := readFile(t, "../../postinstall.js")
	for _, want := range []string{
		`project_name: biblical-atlas-mcp`,
		`name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`,
		`goos: [linux, darwin, windows]`,
		`goarch: [amd64, arm64]`,
		`formats: [tar.gz]`,
		`formats: [zip]`,
		`name_template: checksums.txt`,
	} {
		if !strings.Contains(gr, want) {
			t.Errorf(".goreleaser.yaml lacks %s", want)
		}
	}
	for _, want := range []string{
		"${NAME}_${VERSION}_${platform}_${arch}.${ext}",
		`const NAME = "biblical-atlas-mcp"`,
		`darwin: "darwin", linux: "linux", win32: "windows"`,
		`x64: "amd64", arm64: "arm64"`,
		`platform === "windows" ? "zip" : "tar.gz"`,
		"/checksums.txt",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("postinstall.js lacks %s", want)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(readFile(t, path)), v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
