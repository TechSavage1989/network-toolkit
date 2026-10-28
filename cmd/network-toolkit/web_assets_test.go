package main

import (
	"bytes"
	"context"
	"html"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type pageDependency struct {
	reference   string
	contentType string
}

// Read the actual document tags rather than maintaining a second asset list.
// Attribute order, quote style and multiline tags do not affect discovery.
func localPageDependencies(t *testing.T, document string) []pageDependency {
	t.Helper()
	tags := regexp.MustCompile(`(?is)<(script|link)\b([^>]*)>`)
	attributes := regexp.MustCompile(`(?is)([a-z][a-z0-9_:-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))`)
	dependencies := []pageDependency{}
	for _, tag := range tags.FindAllStringSubmatch(document, -1) {
		values := map[string]string{}
		for _, attribute := range attributes.FindAllStringSubmatch(tag[2], -1) {
			values[strings.ToLower(attribute[1])] = html.UnescapeString(attribute[2] + attribute[3] + attribute[4])
		}
		dependency := pageDependency{reference: values["src"], contentType: "text/javascript"}
		if strings.EqualFold(tag[1], "link") {
			stylesheet := false
			for _, relation := range strings.Fields(strings.ToLower(values["rel"])) {
				stylesheet = stylesheet || relation == "stylesheet"
			}
			if !stylesheet {
				continue
			}
			dependency = pageDependency{reference: values["href"], contentType: "text/css"}
		}
		if dependency.reference == "" {
			continue
		}
		reference, err := url.Parse(dependency.reference)
		if err != nil {
			t.Fatalf("invalid page dependency %q: %v", dependency.reference, err)
		}
		if reference.IsAbs() || reference.Host != "" {
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies
}

func TestEveryHTMLDependencyIsServedFromMainAndDetachedRoutes(t *testing.T) {
	a := testApplication(t)
	source, err := web.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	expected := localPageDependencies(t, embeddedAssetPaths(string(source)))
	if len(expected) == 0 {
		t.Fatal("no local scripts or stylesheets were discovered in the embedded document")
	}
	// An empty linked workspace exercises the real detached page route without
	// connecting a terminal or launching an external application.
	view, _ := workspaceTestCreate(t, a, terminalWorkspaceSnapshot{})
	a.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	_, _, editorID := editorFixtureOffer(t, a.editorViews)
	for _, route := range []string{"/", "/terminal-view/" + view.ID, "/editor-view/" + editorID} {
		t.Run(route, func(t *testing.T) {
			page := requestApp(a, http.MethodGet, route, "", false)
			if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("interface route %s: %d %s", route, page.Code, page.Header().Get("Content-Type"))
			}
			dependencies := localPageDependencies(t, page.Body.String())
			if len(dependencies) != len(expected) {
				t.Fatalf("interface dependencies changed during rendering: got %d, want %d", len(dependencies), len(expected))
			}
			base, err := url.Parse("http://" + a.host + route)
			if err != nil {
				t.Fatal(err)
			}
			for index, dependency := range dependencies {
				t.Run(dependency.reference, func(t *testing.T) {
					if dependency != expected[index] {
						t.Fatalf("rendered dependency differs from normalized embedded HTML: %#v, want %#v", dependency, expected[index])
					}
					reference, err := url.Parse(dependency.reference)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.HasPrefix(reference.Path, "/assets/") {
						t.Fatalf("dependency %q is not rooted and can resolve inside a detached page route", dependency.reference)
					}
					resolved := base.ResolveReference(reference)
					response := requestApp(a, http.MethodGet, resolved.RequestURI(), "", false)
					if response.Code != http.StatusOK {
						t.Fatalf("HTML dependency %s was not served through the application: %d", resolved.Path, response.Code)
					}
					contentType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
					if err != nil || contentType != dependency.contentType {
						t.Fatalf("dependency %s MIME type %q, want %s: %v", resolved.Path, response.Header().Get("Content-Type"), dependency.contentType, err)
					}
					embedded, err := web.ReadFile("web" + resolved.Path)
					if err != nil || len(embedded) == 0 || !bytes.Equal(embedded, response.Body.Bytes()) {
						t.Fatalf("dependency %s did not serve its exact embedded bytes: %v", resolved.Path, err)
					}
				})
			}
		})
	}
	for _, path := range []string{"/assets/", "/assets/not-a-public-asset.js", "/assets/source.go", "/assets/../main.go", "/assets/SOURCE.json", "/assets/terminal-macro-model.js.map", "/assets/app.manifest", "/web/index.html", "/terminal-view/" + view.ID + "/assets/terminal-macros.js"} {
		if response := requestApp(a, http.MethodGet, path, "", false); response.Code != http.StatusNotFound {
			t.Errorf("non-public path %s was exposed: %d", path, response.Code)
		}
	}
}

func TestApplicationIconAssets(t *testing.T) {
	a := testApplication(t)
	for _, item := range []struct{ path, source, contentType string }{
		{"/assets/Network_Toolkit_Icon.svg", "assets/Network_Toolkit_Icon.svg", "image/svg+xml"},
		{"/assets/Network_Toolkit_Icon.ico", "assets/Network_Toolkit_Icon.ico", "image/x-icon"},
		{"/favicon.ico", "assets/Network_Toolkit_Icon.ico", "image/x-icon"},
	} {
		original, err := os.ReadFile("../../" + item.source)
		if err != nil {
			t.Fatal(err)
		}
		response := requestApp(a, http.MethodGet, item.path, "", false)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != item.contentType || !bytes.Equal(response.Body.Bytes(), original) {
			t.Fatalf("icon %s did not serve its original artwork: %d", item.path, response.Code)
		}
		head := requestApp(a, http.MethodHead, item.path, "", false)
		if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != item.contentType {
			t.Fatalf("icon HEAD %s: %d, %d bytes", item.path, head.Code, head.Body.Len())
		}
		if post := requestApp(a, http.MethodPost, item.path, "{}", false); post.Code != http.StatusMethodNotAllowed {
			t.Fatalf("icon POST %s: %d", item.path, post.Code)
		}
		badHost := httptest.NewRequest(http.MethodGet, "http://attacker.invalid"+item.path, nil)
		rejected := httptest.NewRecorder()
		a.ServeHTTP(rejected, badHost)
		if rejected.Code != http.StatusForbidden {
			t.Fatalf("icon host restriction %s: %d", item.path, rejected.Code)
		}
	}
	for _, mode := range []string{"main", "terminal"} {
		a.windowMode = mode
		html := requestApp(a, http.MethodGet, "/", "", false).Body.String()
		for _, name := range []string{"Network_Toolkit_Icon.svg", "Network_Toolkit_Icon.ico"} {
			if !strings.Contains(html, `"/assets/`+name+`"`) || strings.Contains(html, `"../assets/`+name+`"`) {
				t.Fatalf("%s interface icon was not rooted: %s", mode, name)
			}
		}
	}
	for _, ref := range []string{"../assets/Network_Toolkit_Icon.svg", "../assets/Network_Toolkit_Icon.ico"} {
		if _, err := os.Stat(filepath.Join("../../web", filepath.FromSlash(ref))); err != nil {
			t.Fatalf("offline icon path %s: %v", ref, err)
		}
	}
	if result := requestApp(a, http.MethodGet, "/assets/app.manifest", "", false); result.Code != http.StatusNotFound {
		t.Fatal("icon route exposed an unrelated asset")
	}
}

func TestDiscoveryAssetsAreEmbeddedAndRooted(t *testing.T) {
	a := testApplication(t)
	for _, item := range []struct{ path, contentType string }{
		{"/assets/scan-scopes.js", "text/javascript"},
		{"/assets/discovery-workspace.js", "text/javascript"},
		{"/assets/discovery-filter.js", "text/javascript"},
		{"/assets/discovery-port-actions.js", "text/javascript"},
		{"/assets/discovery-workspace.css", "text/css"},
		{"/assets/source-interface.js", "text/javascript"},
		{"/assets/source-interface.css", "text/css"},
		{"/assets/result-details.js", "text/javascript"},
		{"/assets/result-details.css", "text/css"},
		{"/assets/scan-routes.js", "text/javascript"},
		{"/assets/scan-routes.css", "text/css"},
		{"/assets/app-navigation.js", "text/javascript"},
		{"/assets/app-navigation.css", "text/css"},
		{"/assets/app-chrome.js", "text/javascript"},
		{"/assets/app-chrome.css", "text/css"},
		{"/assets/sidebar-activity.js", "text/javascript"},
		{"/assets/sidebar-activity.css", "text/css"},
		{"/assets/network-config-language.js", "text/javascript"},
		{"/assets/editor-compare.js", "text/javascript"},
		{"/assets/editor-windows.js", "text/javascript"},
		{"/assets/editor-windows.css", "text/css"},
		{"/assets/configuration-layout.js", "text/javascript"},
		{"/assets/configuration-layout.css", "text/css"},
		{"/assets/configuration-surfaces.css", "text/css"},
		{"/assets/terminal-interaction.js", "text/javascript"},
		{"/assets/terminal-interaction.css", "text/css"},
		{"/assets/terminal-copy.js", "text/javascript"},
		{"/assets/terminal-copy.css", "text/css"},
		{"/assets/terminal-output-archive.js", "text/javascript"},
		{"/assets/terminal-tabs.js", "text/javascript"},
		{"/assets/packet-capture.js", "text/javascript"},
		{"/assets/packet-capture.css", "text/css"},
		{"/assets/input-clear.js", "text/javascript"},
		{"/assets/input-clear.css", "text/css"},
		{"/assets/transfer-locations.js", "text/javascript"},
		{"/assets/value-copy.js", "text/javascript"},
		{"/assets/value-copy.css", "text/css"},
		{"/assets/responsive-controls.css", "text/css"},
	} {
		response := requestApp(a, http.MethodGet, item.path, "", false)
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), item.contentType) {
			t.Fatalf("asset %s status/type: %d %s", item.path, response.Code, response.Header().Get("Content-Type"))
		}
		embedded, err := web.ReadFile("web" + item.path)
		if err != nil || !bytes.Equal(embedded, response.Body.Bytes()) {
			t.Fatalf("asset %s differs from embedded source: %v", item.path, err)
		}
		head := requestApp(a, http.MethodHead, item.path, "", false)
		if head.Code != http.StatusOK || head.Body.Len() != 0 {
			t.Fatalf("asset HEAD %s: %d, %d bytes", item.path, head.Code, head.Body.Len())
		}
		if post := requestApp(a, http.MethodPost, item.path, "{}", false); post.Code != http.StatusMethodNotAllowed {
			t.Fatalf("asset POST %s: %d", item.path, post.Code)
		}
	}
	html := requestApp(a, http.MethodGet, "/", "", false).Body.String()
	for _, path := range []string{"/assets/scan-scopes.js", "/assets/discovery-workspace.js", "/assets/discovery-workspace.css", "/assets/source-interface.js", "/assets/source-interface.css", "/assets/result-details.js", "/assets/result-details.css", "/assets/scan-routes.js", "/assets/scan-routes.css", "/assets/app-navigation.js", "/assets/app-navigation.css", "/assets/configuration-layout.js", "/assets/configuration-layout.css", "/assets/configuration-surfaces.css", "/assets/terminal-interaction.js", "/assets/terminal-interaction.css", "/assets/terminal-copy.js", "/assets/terminal-copy.css"} {
		if !strings.Contains(html, `"`+path+`"`) {
			t.Fatalf("rooted interface asset missing: %s", path)
		}
	}
	if strings.Contains(html, `src="assets/`) || strings.Contains(html, `href="assets/`) {
		t.Fatal("relative interface assets would break a detached route")
	}
	if response := requestApp(a, http.MethodGet, "/assets/source.go", "", false); response.Code != http.StatusNotFound {
		t.Fatal("asset allowlist exposed an unknown file")
	}
}

func TestEditorWorkerDependenciesAreEmbeddedAndServed(t *testing.T) {
	a := testApplication(t)
	source, err := web.ReadFile("web/assets/editor-language-worker.js")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"/assets/editor-language-worker.js"}
	for _, match := range regexp.MustCompile(`['"](editor-vendor/[^'"]+\.js)['"]`).FindAllStringSubmatch(string(source), -1) {
		paths = append(paths, "/assets/"+match[1])
	}
	if len(paths) < 10 {
		t.Fatal("worker imports were not discovered")
	}
	for _, path := range paths {
		response := requestApp(a, http.MethodGet, path, "", false)
		expected, readErr := web.ReadFile("web" + path)
		if readErr != nil || response.Code != http.StatusOK || !bytes.Equal(expected, response.Body.Bytes()) || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/javascript") {
			t.Fatalf("worker dependency %s is unavailable: %d %v", path, response.Code, readErr)
		}
	}
	for _, path := range []string{"/assets/editor-vendor/", "/assets/editor-vendor/SOURCE.json", "/assets/editor-vendor/prettier-NOTICES.txt", "/assets/editor-vendor/not-a-language.js"} {
		if response := requestApp(a, http.MethodGet, path, "", false); response.Code != http.StatusNotFound {
			t.Fatalf("non-public worker path exposed: %s", path)
		}
	}
}
