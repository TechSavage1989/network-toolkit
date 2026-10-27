package desktop

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// This script is installed by the native host before the first navigation.
// The HTTP response never carries the capability. Keep all interpolated values
// JSON encoded, and check the top frame and the exact document, not just origin.
const nativeSessionBootstrapTemplate = `(()=>{
'use strict';
if(window.top!==window||location.origin!==__ORIGIN__||location.pathname!==__PATH__||location.search!=='')return;
Object.defineProperty(window,'__networkToolkitNativeSession',{value:__TOKEN__,writable:false,configurable:false,enumerable:false});
})();`

func nativeDocumentURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("The native interface requires an exact local document URL")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || u.Host != "127.0.0.1:"+strconv.Itoa(port) || u.EscapedPath() != u.Path {
		return nil, errors.New("The native interface URL is not canonical")
	}
	if u.Path != "/" {
		id := strings.TrimPrefix(u.Path, "/terminal-view/")
		decoded, err := hex.DecodeString(id)
		if id == u.Path || err != nil || len(decoded) != 16 || strings.ToLower(id) != id {
			return nil, errors.New("The native interface document is not an application workspace")
		}
	}
	return u, nil
}

func sameNativeDocument(appURL, candidate string) bool {
	app, err := nativeDocumentURL(appURL)
	if err != nil {
		return false
	}
	target, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	// In-page fragments are harmless; no query, path change, userinfo, alias or
	// escaped path is accepted as an alternate capability-bearing document.
	target.Fragment, target.RawFragment = "", ""
	target, err = nativeDocumentURL(target.String())
	return err == nil && target.Scheme == app.Scheme && target.Host == app.Host && target.Path == app.Path
}

func nativeSessionBootstrap(appURL, token string) (string, error) {
	u, err := nativeDocumentURL(appURL)
	if err != nil {
		return "", err
	}
	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 32 || strings.ToLower(token) != token {
		return "", errors.New("The native interface capability is unavailable")
	}
	origin, _ := json.Marshal(u.Scheme + "://" + u.Host)
	path, _ := json.Marshal(u.Path)
	secret, _ := json.Marshal(token)
	return strings.NewReplacer("__ORIGIN__", string(origin), "__PATH__", string(path), "__TOKEN__", string(secret)).Replace(nativeSessionBootstrapTemplate), nil
}
