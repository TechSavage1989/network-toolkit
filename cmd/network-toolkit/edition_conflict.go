//go:build ntk_editor && ntk_terminal

package main

// The two tags build different programs; a build with both must not compile.
const buildEdition = ntk_editor_and_ntk_terminal_build_different_programs
