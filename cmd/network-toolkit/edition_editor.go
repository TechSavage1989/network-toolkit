//go:build ntk_editor && !ntk_terminal

package main

// Network_Toolkit_TextEditor.exe: the standalone Text Editor without a switch
// (edition.go). Its Windows resources replace the main program's, which the
// build leaves out with an overlay (build.sh).
import _ "networktoolkit/internal/winres/editor"

const buildEdition = standaloneEditorMode
