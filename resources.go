// Package networktoolkit provides the immutable assets embedded by the application.
package networktoolkit

import "embed"

// Web preserves the application's embedded paths independently of its command directory.
//
//go:embed web/index.html web/assets/*.js web/assets/*.css web/assets/editor-vendor web/assets/vendor/codemirror assets/Network_Toolkit_Icon.svg assets/Network_Toolkit_Icon.ico
var Web embed.FS
