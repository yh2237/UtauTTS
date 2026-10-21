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

func TestPublicWebsiteWithoutEditorLink(t *testing.T) {
	source, output := fixture(t)
	site := t.TempDir()
	const index = "<!DOCTYPE html><html><head><title>Website</title></head><body>Profile</body></html>"
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte(index), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := assemble(source, output, "https://assets.example.test", "site", "v1.2.3", strings.Repeat("a", 40), "previews", site)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, html, editor, wantError string
	}{
		{name: "website without editor link", html: index},
		{name: "non-HTML homepage", html: "not a website", wantError: "HTML page"},
		{name: "wrong editor runtime", html: index, editor: "<html>wrong runtime</html>", wantError: "packaged runtime"},
	} {
		t.Run(test.name, func(t *testing.T) {
			pages := http.FileServer(http.Dir(filepath.Join(output, "pages")))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					fmt.Fprint(w, test.html)
					return
				}
				if r.URL.Path == "/editor/" && test.editor != "" {
					fmt.Fprint(w, test.editor)
					return
				}
				if strings.HasPrefix(r.URL.Path, "/r2/") {
					for _, asset := range m.Assets {
						if r.URL.Path == "/r2/"+asset.Path {
							w.Header().Set("Access-Control-Allow-Origin", "*")
							w.Header().Set("Content-Type", asset.ContentType)
							w.Header().Set("Content-Length", fmt.Sprint(asset.Size))
							return
						}
					}
					http.NotFound(w, r)
					return
				}
				pages.ServeHTTP(w, r)
			}))
			defer server.Close()
			m.AssetBaseURL = server.URL + "/r2/"
			if err := writeJSON(filepath.Join(output, "upload-manifest.json"), m); err != nil {
				t.Fatal(err)
			}
			err := checkPublic(output, server.URL, server.Client())
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected %q, got %v", test.wantError, err)
			}
		})
	}
}
