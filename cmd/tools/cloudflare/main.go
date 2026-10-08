package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"utautts/cmd/tools/internal/toolutil"
)

var hostFiles = []string{"config.js", "asset-paths.js", "bootstrap.js", "engine-loader.js", "engine-worker.js", "utautts.js", "qtloader.js"}

const pageLimit = 25 << 20
const immutableCache = "public, max-age=31536000, immutable"

type asset struct {
	Path        string `json:"path"`
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
}

type manifest struct {
	Version      string  `json:"version"`
	Revision     string  `json:"revision"`
	Channel      string  `json:"channel"`
	DeploymentID string  `json:"deployment_id"`
	AssetBaseURL string  `json:"asset_base_url"`
	R2Prefix     string  `json:"r2_prefix"`
	Assets       []asset `json:"assets,omitempty"`
}

func match(pattern, value string) bool { return regexp.MustCompile(pattern).MatchString(value) }
func inside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

// releaseVersionは正式版（vX.X.X）とベータ版（vX.X.X-beta.N）の版の形式。
const releaseVersion = `^v[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$`

func deploymentPlan(event, ref, preview, revision, runID, attempt, version string) (map[string]string, error) {
	if !match(releaseVersion, version) {
		return nil, errors.New("appinfo version must have the form vX.X.X or vX.X.X-beta.N")
	}
	if !match(`^[0-9a-f]{40}$`, revision) {
		return nil, errors.New("revision must be a full git SHA")
	}
	if !match(`^[0-9]+$`, runID) || !match(`^[0-9]+$`, attempt) {
		return nil, errors.New("run id and attempt must be numeric")
	}
	tagged := event == "push" && strings.HasPrefix(ref, "refs/tags/")
	production := false
	label := preview
	if tagged {
		tag := strings.TrimPrefix(ref, "refs/tags/")
		if !match(releaseVersion, tag) {
			return nil, errors.New("release tag must have the form vX.X.X or vX.X.X-beta.N")
		}
		if tag != version {
			return nil, fmt.Errorf("tag %s differs from appinfo version %s", tag, version)
		}
		// ベータ版は本番ではなく、タグ名のプレビューへ出す。
		production = !strings.Contains(tag, "-")
		label = tag
		if !production {
			label = strings.NewReplacer(".", "-").Replace(tag)
		}
	} else if event == "workflow_dispatch" {
		if !match(`^[a-z0-9][a-z0-9-]{0,39}$`, preview) {
			return nil, errors.New("preview name must be 1-40 lowercase letters, digits or hyphens")
		}
	} else {
		return nil, errors.New("deployments require a release tag or a manual preview")
	}
	channel, branch := "previews", "preview-"+label
	if production {
		channel, branch = "releases", "main"
	}
	return map[string]string{"channel": channel, "branch": branch, "deployment_id": fmt.Sprintf("%s-%s-%s-%s", label, revision[:12], runID, attempt), "version": version, "revision": revision}, nil
}

func readJSON(file string, value any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(strings.TrimPrefix(string(data), "\ufeff")), value)
}
func writeJSON(file string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0644)
}
func checkedFile(root, relative string) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	file, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("missing or invalid release file: %s", relative)
	}
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("missing or invalid release file: %s", relative)
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	realFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(realBase, realFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("missing or invalid release file: %s", relative)
	}
	return file, nil
}
func assetFiles(source string) ([]string, error) {
	files := []string{"utautts.wasm", "renderer/utautts-world-phrase/renderer.json"}
	for _, name := range []string{"utautts.wasm", "wasm_exec.js", "fs-shim.js", "openjtalk-bridge.js", "world-bridge.js", "openjtalk/utautts-openjtalk.js", "openjtalk/utautts-openjtalk.wasm", "world/utautts-world.js", "world/utautts-world.wasm", "openjtalk/dict-manifest.json", "models/manifest.json", "voice/manifest.json"} {
		files = append(files, "web/dist/"+name)
	}
	for _, spec := range []struct{ manifest, field, prefix string }{
		{"web/dist/openjtalk/dict-manifest.json", "files", "web/dist/openjtalk/dict/"},
		{"web/dist/models/manifest.json", "models", "web/dist/models/"},
		{"web/dist/voice/manifest.json", "files", "web/dist/voice/"},
	} {
		file, err := checkedFile(source, spec.manifest)
		if err != nil {
			return nil, err
		}
		var values map[string]json.RawMessage
		if err := readJSON(file, &values); err != nil {
			return nil, err
		}
		var names []string
		if err := json.Unmarshal(values[spec.field], &names); err != nil {
			var named map[string]json.RawMessage
			if err = json.Unmarshal(values[spec.field], &named); err != nil {
				return nil, err
			}
			for name := range named {
				names = append(names, name)
			}
		}
		for _, name := range names {
			files = append(files, spec.prefix+name)
		}
	}
	sort.Strings(files)
	return files, nil
}
func contentType(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".wasm":
		return "application/wasm"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".wav":
		return "audio/wav"
	}
	if kind := mime.TypeByExtension(filepath.Ext(file)); kind != "" {
		return kind
	}
	return "application/octet-stream"
}
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
func assemble(source, output, publicURL, id, version, revision, channel string) (manifest, error) {
	var result manifest
	if err := toolutil.RequireUnderOut(output, "output", false); err != nil {
		return result, err
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		return result, err
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return result, err
	}
	if inside(absSource, absOutput) || inside(absOutput, absSource) {
		return result, fmt.Errorf("source and output must be separate directories")
	}
	if _, err := os.Stat(output); err == nil {
		return result, fmt.Errorf("refusing to overwrite %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if !match(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,159}$`, id) {
		return result, errors.New("deployment id must be a single URL-safe path component")
	}
	if channel != "releases" && channel != "previews" {
		return result, errors.New("invalid channel")
	}
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return result, errors.New("R2 public URL must be an absolute HTTP(S) directory URL without credentials")
	}
	deps, err := assetFiles(source)
	if err != nil {
		return result, err
	}
	for _, name := range append(append(append([]string{}, hostFiles...), "index.html"), deps...) {
		if _, err := checkedFile(source, name); err != nil {
			return result, err
		}
	}
	result = manifest{Version: version, Revision: revision, Channel: channel, DeploymentID: id, R2Prefix: "utautts/" + channel + "/" + id}
	result.AssetBaseURL = strings.TrimRight(publicURL, "/") + "/" + result.R2Prefix + "/"
	app := filepath.Join(output, "pages", "app", id)
	if err := os.MkdirAll(app, 0755); err != nil {
		return result, err
	}
	for _, name := range hostFiles {
		if err := copyFile(filepath.Join(source, name), filepath.Join(app, name)); err != nil {
			return result, err
		}
	}
	config, _ := json.Marshal(map[string]string{"assetBaseURL": result.AssetBaseURL})
	if err := os.WriteFile(filepath.Join(app, "config.js"), []byte("globalThis.UtauTTSConfig = "+strings.Replace(string(config), ":\"", ": \"", 1)+";\n"), 0644); err != nil {
		return result, err
	}
	html, err := os.ReadFile(filepath.Join(source, "index.html"))
	if err != nil {
		return result, err
	}
	page := string(html)
	for _, name := range []string{"config.js", "asset-paths.js", "bootstrap.js"} {
		old := `src="./` + name + `"`
		if !strings.Contains(page, old) {
			return result, fmt.Errorf("index.html is missing %s", old)
		}
		page = strings.ReplaceAll(page, old, `src="./app/`+id+`/`+name+`"`)
	}
	pages := filepath.Join(output, "pages")
	for file, value := range map[string]string{"index.html": page, "404.html": "<!doctype html><meta charset=utf-8><title>404</title>Not found\n", "_headers": "/*\n  X-Content-Type-Options: nosniff\n/\n  Cache-Control: no-cache\n/index.html\n  Cache-Control: no-cache\n/deployment.json\n  Cache-Control: no-cache\n/app/*\n  Cache-Control: " + immutableCache + "\n"} {
		if err := os.WriteFile(filepath.Join(pages, file), []byte(value), 0644); err != nil {
			return result, err
		}
	}
	for _, name := range deps {
		file, _ := checkedFile(source, name)
		data, err := os.ReadFile(file)
		if err != nil {
			return result, err
		}
		dst := filepath.Join(output, "r2", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return result, err
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return result, err
		}
		sum := sha256.Sum256(data)
		result.Assets = append(result.Assets, asset{name, result.R2Prefix + "/" + name, int64(len(data)), hex.EncodeToString(sum[:]), contentType(file)})
	}
	if err := writeJSON(filepath.Join(output, "upload-manifest.json"), result); err != nil {
		return result, err
	}
	public := result
	public.Assets = nil
	if err := writeJSON(filepath.Join(pages, "deployment.json"), public); err != nil {
		return result, err
	}
	err = filepath.Walk(pages, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Size() > pageLimit {
			return fmt.Errorf("Pages asset exceeds 25 MiB: %s", file)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	fmt.Printf("R2: %d files\nAsset URL: %s\n", len(result.Assets), result.AssetBaseURL)
	return result, nil
}
func loadManifest(output string) (manifest, error) {
	var m manifest
	err := readJSON(filepath.Join(output, "upload-manifest.json"), &m)
	return m, err
}
func requestPublic(client *http.Client, address, method, origin string) (http.Header, []byte, error) {
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequest(method, address, nil)
		if err != nil {
			return nil, nil, err
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && readErr == nil {
				return resp.Header, body, nil
			}
			last = fmt.Errorf("%s: HTTP %d: %v", address, resp.StatusCode, readErr)
		} else {
			last = err
		}
		if attempt < 4 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	return nil, nil, fmt.Errorf("public asset check failed: %s: %w", address, last)
}
func checkAssets(output, origin string, client *http.Client) error {
	m, err := loadManifest(output)
	if err != nil {
		return err
	}
	samples := map[string]bool{"utautts.wasm": true, "web/dist/utautts.wasm": true, "web/dist/fs-shim.js": true, "web/dist/openjtalk/dict-manifest.json": true, "web/dist/models/manifest.json": true, "web/dist/voice/manifest.json": true, "renderer/utautts-world-phrase/renderer.json": true, "web/dist/openjtalk/utautts-openjtalk.wasm": true, "web/dist/world/utautts-world.wasm": true}
	voice, dict := false, false
	for _, a := range m.Assets {
		if !voice && strings.HasPrefix(a.Path, "web/dist/voice/") && strings.HasSuffix(a.Path, ".wav") {
			samples[a.Path] = true
			voice = true
		}
		if !dict && strings.HasPrefix(a.Path, "web/dist/openjtalk/dict/") {
			samples[a.Path] = true
			dict = true
		}
	}
	if !voice || !dict {
		return errors.New("manifest lacks voice WAV or dictionary")
	}
	for _, a := range m.Assets {
		if !samples[a.Path] {
			continue
		}
		u := m.AssetBaseURL + (&url.URL{Path: a.Path}).EscapedPath()
		h, _, err := requestPublic(client, u, "HEAD", origin)
		if err != nil {
			return err
		}
		if cors := h.Get("Access-Control-Allow-Origin"); cors != "*" && cors != origin {
			return fmt.Errorf("R2 CORS does not allow %s: %s", origin, u)
		}
		if strings.Split(h.Get("Content-Type"), ";")[0] != strings.Split(a.ContentType, ";")[0] {
			return fmt.Errorf("incorrect Content-Type for %s: %s", u, h.Get("Content-Type"))
		}
		if h.Get("Content-Length") != strconv.FormatInt(a.Size, 10) {
			return fmt.Errorf("incorrect Content-Length for %s", u)
		}
	}
	fmt.Printf("R2 public assets verified for %s\n", origin)
	return nil
}
func checkPublic(output, pagesURL string, client *http.Client) error {
	m, err := loadManifest(output)
	if err != nil {
		return err
	}
	u, err := url.Parse(pagesURL)
	if err != nil {
		return err
	}
	origin := u.Scheme + "://" + u.Host
	base := strings.TrimRight(pagesURL, "/") + "/"
	_, data, err := requestPublic(client, base+"deployment.json", "GET", "")
	if err != nil {
		return err
	}
	var served manifest
	if err := json.Unmarshal(data, &served); err != nil {
		return err
	}
	if served.DeploymentID != m.DeploymentID {
		return errors.New("Pages is serving a different deployment")
	}
	_, html, err := requestPublic(client, base, "GET", "")
	if err != nil {
		return err
	}
	if !strings.Contains(string(html), "app/"+m.DeploymentID+"/bootstrap.js") {
		return errors.New("Pages entry point does not reference the packaged runtime")
	}
	for _, name := range hostFiles {
		if _, _, err := requestPublic(client, base+path.Join("app", m.DeploymentID, name), "HEAD", ""); err != nil {
			return err
		}
	}
	if err := checkAssets(output, origin, client); err != nil {
		return err
	}
	fmt.Printf("Pages entry point and R2 public assets verified: %s\n", base)
	return nil
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("expected plan, package, upload, verify, or verify-assets")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "plan":
		event := f.String("event", "", "")
		ref := f.String("ref", "", "")
		preview := f.String("preview-name", "dev", "")
		revision := f.String("revision", "", "")
		runID := f.String("run-id", "", "")
		attempt := f.String("attempt", "", "")
		appinfo := f.String("appinfo", "internal/appinfo/appinfo.json", "")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		var app map[string]any
		if err := readJSON(*appinfo, &app); err != nil {
			return err
		}
		values, err := deploymentPlan(*event, *ref, *preview, *revision, *runID, *attempt, fmt.Sprint(app["version"]))
		if err != nil {
			return err
		}
		for _, key := range []string{"channel", "branch", "deployment_id", "version", "revision"} {
			fmt.Printf("%s=%s\n", key, values[key])
		}
		return nil
	case "package":
		source := f.String("source", "build/web-release", "")
		output := f.String("output", "out/cloudflare", "")
		publicURL := f.String("public-url", "", "")
		id := f.String("deployment-id", "", "")
		version := f.String("version", "", "")
		revision := f.String("revision", "", "")
		channel := f.String("channel", "", "")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		_, err := assemble(*source, *output, *publicURL, *id, *version, *revision, *channel)
		return err
	case "upload", "verify", "verify-assets":
		output := f.String("output", "out/cloudflare", "")
		bucket := f.String("bucket", os.Getenv("CF_R2_BUCKET"), "")
		account := f.String("account-id", os.Getenv("CLOUDFLARE_ACCOUNT_ID"), "")
		pagesURL := f.String("pages-url", "", "")
		origin := f.String("origin", "", "")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if args[0] == "upload" {
			return upload(*output, *bucket, *account)
		}
		client := &http.Client{Timeout: 60 * time.Second}
		if args[0] == "verify" {
			return checkPublic(*output, *pagesURL, client)
		}
		return checkAssets(*output, *origin, client)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
