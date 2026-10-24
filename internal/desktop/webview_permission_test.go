package desktop

import (
	"errors"
	"reflect"
	"testing"
)

type permissionFixture struct {
	requestURI        string
	permissionKind    int32
	gesture           bool
	uriError          error
	kindError         error
	gestureError      error
	persistenceError  error
	initialDenyError  error
	allowError        error
	fallbackDenyError error
	states            []int32
	metadataReads     int
	persistenceCalls  int
	afterPersistence  func()
}

func (f *permissionFixture) uri() (string, error) {
	f.metadataReads++
	return f.requestURI, f.uriError
}
func (f *permissionFixture) kind() (int32, error) {
	f.metadataReads++
	return f.permissionKind, f.kindError
}
func (f *permissionFixture) userInitiated() (bool, error) {
	f.metadataReads++
	return f.gesture, f.gestureError
}
func (f *permissionFixture) disablePersistence() error {
	f.persistenceCalls++
	if f.afterPersistence != nil {
		f.afterPersistence()
	}
	return f.persistenceError
}
func (f *permissionFixture) setState(state int32) error {
	f.states = append(f.states, state)
	if state == webviewPermissionAllow {
		return f.allowError
	}
	if len(f.states) == 1 {
		return f.initialDenyError
	}
	return f.fallbackDenyError
}
func newPermissionFixture() *permissionFixture {
	return &permissionFixture{requestURI: "http://127.0.0.1:45678/", permissionKind: webviewPermissionClipboardRead, gesture: true}
}

func TestClipboardPermissionOriginAndGesturePolicy(t *testing.T) {
	const app = "http://127.0.0.1:45678/"
	for _, uri := range []string{app, app + "?terminal=detached", app + "#terminal"} {
		f := newPermissionFixture()
		f.requestURI = uri
		if err := applyWebViewPermission(app, func() bool { return false }, f); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny, webviewPermissionAllow}) || f.persistenceCalls != 1 {
			t.Fatalf("explicit same-origin clipboard request = %+v", f)
		}
	}
	for _, uri := range []string{
		"", "https://127.0.0.1:45678/", "http://127.0.0.1:45679/", "http://localhost:45678/", "http://[::1]:45678/",
		"http://user@127.0.0.1:45678/", "http://127.0.0.1:45678.evil.test/", "http://127.0.0.1:45678@evil.test/",
		"http://192.0.2.1:45678/", "file:///C:/report.html", "data:text/plain,test", "about:blank", "blob:http://127.0.0.1:45678/report",
	} {
		f := newPermissionFixture()
		f.requestURI = uri
		if err := applyWebViewPermission(app, func() bool { return false }, f); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) || f.persistenceCalls != 0 {
			t.Errorf("foreign origin %q was not denied: %+v", uri, f)
		}
	}
	for kind := int32(-1); kind <= 20; kind++ {
		if kind == webviewPermissionClipboardRead {
			continue
		}
		f := newPermissionFixture()
		f.permissionKind = kind
		if err := applyWebViewPermission(app, func() bool { return false }, f); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) || f.persistenceCalls != 0 {
			t.Errorf("other permission %d was not denied: %+v", kind, f)
		}
	}
	f := newPermissionFixture()
	f.gesture = false
	if err := applyWebViewPermission(app, func() bool { return false }, f); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) || f.persistenceCalls != 1 {
		t.Fatalf("background clipboard read must be denied without caching: %+v", f)
	}
}

func TestClipboardPermissionCOMFailuresRemainDenied(t *testing.T) {
	const app = "http://127.0.0.1:45678/"
	failure := errors.New("synthetic native failure")
	for _, test := range []struct {
		name string
		fail func(*permissionFixture)
	}{
		{"URI getter", func(f *permissionFixture) { f.uriError = failure }},
		{"kind getter", func(f *permissionFixture) { f.kindError = failure }},
		{"gesture getter", func(f *permissionFixture) { f.gestureError = failure }},
		{"missing Args3 interface", func(f *permissionFixture) { f.persistenceError = errors.New("E_NOINTERFACE") }},
		{"persistence setter", func(f *permissionFixture) { f.persistenceError = failure }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newPermissionFixture()
			test.fail(f)
			if err := applyWebViewPermission(app, func() bool { return false }, f); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) {
				t.Fatalf("metadata/runtime failure allowed clipboard access: %+v", f)
			}
		})
	}
	f := newPermissionFixture()
	f.initialDenyError = failure
	if err := applyWebViewPermission(app, func() bool { return false }, f); !errors.Is(err, failure) || f.metadataReads != 0 || f.persistenceCalls != 0 {
		t.Fatalf("initial deny failure was ignored: fixture=%+v error=%v", f, err)
	}
	f = newPermissionFixture()
	f.allowError = failure
	if err := applyWebViewPermission(app, func() bool { return false }, f); !errors.Is(err, failure) || !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny, webviewPermissionAllow, webviewPermissionDeny}) {
		t.Fatalf("failed allow did not restore denial: fixture=%+v error=%v", f, err)
	}
	fallbackFailure := errors.New("synthetic fallback deny failure")
	f = newPermissionFixture()
	f.allowError, f.fallbackDenyError = failure, fallbackFailure
	if err := applyWebViewPermission(app, func() bool { return false }, f); !errors.Is(err, failure) || !errors.Is(err, fallbackFailure) {
		t.Fatalf("setter failures were not both retained: %v", err)
	}
}

func TestClipboardPermissionClosingWindowStaysDenied(t *testing.T) {
	const app = "http://127.0.0.1:45678/"
	f := newPermissionFixture()
	if err := applyWebViewPermission(app, func() bool { return true }, f); err != nil || f.metadataReads != 0 || !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) {
		t.Fatalf("closed window processed request: fixture=%+v error=%v", f, err)
	}
	closed := false
	f = newPermissionFixture()
	f.afterPersistence = func() { closed = true }
	if err := applyWebViewPermission(app, func() bool { return closed }, f); err != nil || !reflect.DeepEqual(f.states, []int32{webviewPermissionDeny}) {
		t.Fatalf("window closed during native calls was allowed: fixture=%+v error=%v", f, err)
	}
}
