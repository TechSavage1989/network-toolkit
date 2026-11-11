//go:build ntk_terminal && !ntk_editor

package main

// Network_Toolkit_Terminal.exe: the standalone Terminal without a switch
// (edition.go). Its Windows resources replace the main program's, which the
// build leaves out with an overlay (build.sh).
import _ "networktoolkit/internal/winres/terminal"

const buildEdition = "terminal"
