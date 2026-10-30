//go:build windows && !toolkit_native_drag_trace122

package desktop

// Ordinary application builds have no environment or filesystem diagnostic sink.
func nativeFileTraceDiagnosticSink() func(NativeFileDragObservation) { return nil }
