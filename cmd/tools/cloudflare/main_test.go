package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	if err := os.MkdirAll("out", 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.MkdirTemp("out", "cloudflare-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(source) })
	write := func(name, data string) {
		file := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range hostFiles {
		write(name, "// host")
	}
	write("index.html", `<script src="./config.js"></script><script src="./asset-paths.js"></script><script src="./bootstrap.js"></script>`)
	write("web/dist/models/manifest.json", `{"models":["model.json"]}`)
	write("web/dist/openjtalk/dict-manifest.json", `{"files":["sys.dic"]}`)
	write("web/dist/voice/manifest.json", `{"files":{"音声/a.wav":4}}`)
	write("web/dist/models/model.json", "{}")
	write("web/dist/openjtalk/dict/sys.dic", "dict")
	write("web/dist/voice/音声/a.wav", "RIFF")
	files, err := assetFiles(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(source, filepath.FromSlash(name))); os.IsNotExist(err) {
			write(name, "asset")
		}
	}
	output := filepath.Join("out", "cloudflare-test", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.RemoveAll(output); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(output) })
	return source, output
}

func TestPackage(t *testing.T) {
	source, output := fixture(t)
	write := func(name, data string) {
		file := filepath.Join(source, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(file), 0755)
		os.WriteFile(file, []byte(data), 0644)
	}
	write("web/dist/out/private.wav", "private")
	m, err := assemble(source, output, "https://assets.example.test", "v1.2.3-abc-123-1", "v1.2.3", strings.Repeat("a", 40), "releases")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) == 0 || m.Assets[0].Key == "" {
		t.Fatal("empty assets")
	}
	if _, err := os.Stat(filepath.Join(output, "r2", "web", "dist", "out")); !os.IsNotExist(err) {
		t.Fatalf("debug output included: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "pages", "utautts.wasm")); !os.IsNotExist(err) {
		t.Fatal("wasm was copied to Pages")
	}
	if _, err := os.Stat(filepath.Join(output, "r2", "utautts.wasm")); err != nil {
		t.Fatal(err)
	}
	if _, err := assemble(source, output, "https://assets.example.test", "v1.2.3-abc-123-1", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil || !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("expected overwrite refusal: %v", err)
	}
}

func TestPackageInvalid(t *testing.T) {
	source, output := fixture(t)
	for _, tc := range []struct{ url, id string }{{"relative/assets", "valid"}, {"https://secret:token@example.test/", "valid"}, {"https://assets.example.test", "../unsafe"}} {
		_, err := assemble(source, output, tc.url, tc.id, "v1.2.3", strings.Repeat("a", 40), "releases")
		if err == nil {
			t.Fatalf("accepted %v", tc)
		}
	}
	if _, err := assemble(source, source, "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil {
		t.Fatal("accepted output outside out")
	}
	if _, err := assemble(source, filepath.Join(source, "nested"), "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil || !strings.Contains(err.Error(), "separate") {
		t.Fatalf("accepted nested output: %v", err)
	}
	if err := os.Remove(filepath.Join(source, "web/dist/world/utautts-world.wasm")); err != nil {
		t.Fatal(err)
	}
	if _, err := assemble(source, output, "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil || !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("expected missing file: %v", err)
	}
	manifestFile := filepath.Join(source, "web", "dist", "models", "manifest.json")
	if err := os.WriteFile(filepath.Join(source, "web", "dist", "world", "utautts-world.wasm"), []byte("asset"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestFile, []byte(`{"models":["../../../../outside.json"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := assemble(source, output, "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil || !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("expected traversal refusal: %v", err)
	}
}

func TestLargeWasmStaysInR2(t *testing.T) {
	source, output := fixture(t)
	file, err := os.OpenFile(filepath.Join(source, "utautts.wasm"), os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(pageLimit, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := assemble(source, output, "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err != nil {
		t.Fatal(err)
	}
	script, err := os.OpenFile(filepath.Join(source, "utautts.js"), os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := script.Seek(pageLimit, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := script.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	script.Close()
	if _, err := assemble(source, output+"-large", "https://assets.example.test", "valid", "v1.2.3", strings.Repeat("a", 40), "releases"); err == nil || !strings.Contains(err.Error(), "25 MiB") {
		t.Fatalf("expected large Pages rejection: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(output + "-large") })
}

func TestDeploymentPlan(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct{ event, ref, preview, branch string }{{"push", "refs/tags/v1.2.3", "dev", "main"}, {"workflow_dispatch", "refs/heads/main", "touch", "preview-touch"}} {
		plan, err := deploymentPlan(tc.event, tc.ref, tc.preview, sha, "123", "1", "v1.2.3")
		if err != nil || plan["branch"] != tc.branch {
			t.Fatalf("%v: %v", plan, err)
		}
	}
	beta, err := deploymentPlan("push", "refs/tags/v1.2.3-beta.1", "dev", sha, "123", "1", "v1.2.3-beta.1")
	if err != nil || beta["channel"] != "previews" || beta["branch"] != "preview-v1-2-3-beta-1" {
		t.Fatalf("beta tag must go to a preview: %v %v", beta, err)
	}
	for _, version := range []string{"v1.2.3-rc.1", "v1.2.3-beta", "v1.2.3+build.1", "v1.2", "v1.2.3.4", "1.2.3", "v１.2.3"} {
		if _, err := deploymentPlan("workflow_dispatch", "refs/heads/main", "dev", sha, "123", "1", version); err == nil {
			t.Fatal(version)
		}
	}
	for _, tc := range []struct{ event, ref, preview, version string }{{"push", "refs/heads/main", "dev", "v1.2.3"}, {"push", "refs/tags/v1.2.4", "dev", "v1.2.3"}, {"workflow_dispatch", "refs/heads/main", "main;command", "v1.2.3"}} {
		if _, err := deploymentPlan(tc.event, tc.ref, tc.preview, sha, "123", "1", tc.version); err == nil {
			t.Fatal(fmt.Sprint(tc))
		}
	}
}

func TestPublicAssetHeaders(t *testing.T) {
	source, output := fixture(t)
	m, err := assemble(source, output, "https://assets.example.test", "check", "v1.2.3", strings.Repeat("a", 40), "previews")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, item := range m.Assets {
			if strings.HasSuffix(r.URL.Path, "/"+item.Path) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Content-Type", item.ContentType)
				w.Header().Set("Content-Length", fmt.Sprint(item.Size))
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	m.AssetBaseURL = server.URL + "/" + m.R2Prefix + "/"
	if err := writeJSON(filepath.Join(output, "upload-manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	if err := checkAssets(output, "https://tts.pages.dev", server.Client()); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/wasm")
		w.Header().Set("Content-Length", "5")
	}))
	defer bad.Close()
	m.AssetBaseURL = bad.URL + "/" + m.R2Prefix + "/"
	if err := writeJSON(filepath.Join(output, "upload-manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	if err := checkAssets(output, "https://tts.pages.dev", bad.Client()); err == nil || !strings.Contains(err.Error(), "CORS") {
		t.Fatalf("expected CORS rejection: %v", err)
	}
}

// R2のURLはSigV4の規則で符号化する（「$」が生のままだと署名がR2側の計算と合わず403になる）。
func TestR2ObjectURLUsesSigV4Encoding(t *testing.T) {
	address := r2ObjectURL("acct", "bucket", "utautts/previews/x/web/dist/voice/足立レイver3.5.0/$read")
	req, err := http.NewRequest("PUT", address, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "/bucket/utautts/previews/x/web/dist/voice/%E8%B6%B3%E7%AB%8B%E3%83%AC%E3%82%A4ver3.5.0/%24read"
	if got := req.URL.EscapedPath(); got != want {
		t.Fatalf("escaped path = %q, want %q", got, want)
	}
	if got := req.URL.RequestURI(); got != want {
		t.Fatalf("request URI = %q, want %q", got, want)
	}
}
