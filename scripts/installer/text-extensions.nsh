; The one list of file types that Setup offers Network Toolkit Text Editor for.
;
; windows.nsi registers every entry (Default apps and "Open with"),
; verify-windows.ps1 reads this file to check the registration, and the list in
; docs/guide/editor.md is generated from it:
;     node scripts/text-extensions.cjs --write
; scripts/text-extensions.test.cjs fails when any of them differs.
;
; What belongs here: formats that are text by definition and that people edit
; by hand. A "; == Title ==" line starts a group; the guide shows the titles.
; What does not: documents (.rtf, .doc, .pdf), images (.svg included), media,
; archives, programs, shortcuts, and names that are sometimes binary (.key,
; .cer, .crt, .pub, .dat, .bin). Those still open through "Open with", where
; the editor is offered for every file, and a file that is not text is shown
; read-only. Never remove an entry that a released Setup registered: an
; upgrade would leave its "Open with" value behind.

!macro TextExtensions ACTION
  ; == Plain text and logs ==
  !insertmacro ${ACTION} ".txt"
  !insertmacro ${ACTION} ".text"
  !insertmacro ${ACTION} ".log"
  !insertmacro ${ACTION} ".out"
  !insertmacro ${ACTION} ".err"
  !insertmacro ${ACTION} ".nfo"
  !insertmacro ${ACTION} ".list"
  ; == Configuration ==
  !insertmacro ${ACTION} ".ini"
  !insertmacro ${ACTION} ".cfg"
  !insertmacro ${ACTION} ".conf"
  !insertmacro ${ACTION} ".cnf"
  !insertmacro ${ACTION} ".config"
  !insertmacro ${ACTION} ".properties"
  !insertmacro ${ACTION} ".env"
  !insertmacro ${ACTION} ".toml"
  !insertmacro ${ACTION} ".yaml"
  !insertmacro ${ACTION} ".yml"
  !insertmacro ${ACTION} ".editorconfig"
  !insertmacro ${ACTION} ".rules"
  !insertmacro ${ACTION} ".inf"
  !insertmacro ${ACTION} ".reg"
  ; == Network devices ==
  !insertmacro ${ACTION} ".rsc"
  !insertmacro ${ACTION} ".cisco"
  !insertmacro ${ACTION} ".ios"
  !insertmacro ${ACTION} ".junos"
  !insertmacro ${ACTION} ".cli"
  !insertmacro ${ACTION} ".zone"
  ; == Certificates in text form ==
  !insertmacro ${ACTION} ".pem"
  !insertmacro ${ACTION} ".csr"
  ; == Data ==
  !insertmacro ${ACTION} ".json"
  !insertmacro ${ACTION} ".jsonl"
  !insertmacro ${ACTION} ".ndjson"
  !insertmacro ${ACTION} ".jsonc"
  !insertmacro ${ACTION} ".json5"
  !insertmacro ${ACTION} ".xml"
  !insertmacro ${ACTION} ".xsd"
  !insertmacro ${ACTION} ".xsl"
  !insertmacro ${ACTION} ".xslt"
  !insertmacro ${ACTION} ".csv"
  !insertmacro ${ACTION} ".tsv"
  !insertmacro ${ACTION} ".sql"
  ; == Documentation and web ==
  !insertmacro ${ACTION} ".md"
  !insertmacro ${ACTION} ".markdown"
  !insertmacro ${ACTION} ".rst"
  !insertmacro ${ACTION} ".adoc"
  !insertmacro ${ACTION} ".tex"
  !insertmacro ${ACTION} ".html"
  !insertmacro ${ACTION} ".htm"
  !insertmacro ${ACTION} ".xhtml"
  !insertmacro ${ACTION} ".css"
  !insertmacro ${ACTION} ".scss"
  !insertmacro ${ACTION} ".less"
  ; == Scripts ==
  !insertmacro ${ACTION} ".sh"
  !insertmacro ${ACTION} ".bash"
  !insertmacro ${ACTION} ".zsh"
  !insertmacro ${ACTION} ".ksh"
  !insertmacro ${ACTION} ".fish"
  !insertmacro ${ACTION} ".ps1"
  !insertmacro ${ACTION} ".psm1"
  !insertmacro ${ACTION} ".psd1"
  !insertmacro ${ACTION} ".bat"
  !insertmacro ${ACTION} ".cmd"
  !insertmacro ${ACTION} ".vbs"
  !insertmacro ${ACTION} ".awk"
  !insertmacro ${ACTION} ".tcl"
  ; == Source code ==
  !insertmacro ${ACTION} ".py"
  !insertmacro ${ACTION} ".pyw"
  !insertmacro ${ACTION} ".js"
  !insertmacro ${ACTION} ".mjs"
  !insertmacro ${ACTION} ".cjs"
  !insertmacro ${ACTION} ".jsx"
  !insertmacro ${ACTION} ".ts"
  !insertmacro ${ACTION} ".tsx"
  !insertmacro ${ACTION} ".go"
  !insertmacro ${ACTION} ".c"
  !insertmacro ${ACTION} ".h"
  !insertmacro ${ACTION} ".cpp"
  !insertmacro ${ACTION} ".cc"
  !insertmacro ${ACTION} ".cxx"
  !insertmacro ${ACTION} ".hpp"
  !insertmacro ${ACTION} ".hh"
  !insertmacro ${ACTION} ".cs"
  !insertmacro ${ACTION} ".java"
  !insertmacro ${ACTION} ".kt"
  !insertmacro ${ACTION} ".rs"
  !insertmacro ${ACTION} ".rb"
  !insertmacro ${ACTION} ".pl"
  !insertmacro ${ACTION} ".pm"
  !insertmacro ${ACTION} ".php"
  !insertmacro ${ACTION} ".lua"
  !insertmacro ${ACTION} ".swift"
  !insertmacro ${ACTION} ".vb"
  ; == Build and infrastructure ==
  !insertmacro ${ACTION} ".mk"
  !insertmacro ${ACTION} ".mak"
  !insertmacro ${ACTION} ".cmake"
  !insertmacro ${ACTION} ".gradle"
  !insertmacro ${ACTION} ".proto"
  !insertmacro ${ACTION} ".tf"
  !insertmacro ${ACTION} ".tfvars"
  !insertmacro ${ACTION} ".hcl"
  !insertmacro ${ACTION} ".dockerfile"
  ; == Version control ==
  !insertmacro ${ACTION} ".diff"
  !insertmacro ${ACTION} ".patch"
  !insertmacro ${ACTION} ".gitignore"
  !insertmacro ${ACTION} ".gitattributes"
  !insertmacro ${ACTION} ".gitconfig"
  !insertmacro ${ACTION} ".gitmodules"
  !insertmacro ${ACTION} ".dockerignore"
!macroend
