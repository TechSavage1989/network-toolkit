# Network configuration syntax in the editor

The editor can recognize common **text** configurations and apply conservative
syntax colors offline. Choose **Auto** for recognition, a named network language
to override it, or **Plain text** to turn syntax colors off for that document.
The choice is independent for each editor tab. Existing programming-language
associations remain available.

Supported lexical families:

- Aruba AOS-CX (including older ArubaOS-CX headers), ArubaOS-Switch/ProCurve,
  and HPE/H3C Comware.
- Fortinet **FortiOS**, not every Fortinet product or export format.
- SonicWall **SonicOS CLI text**, not an encoded/binary `.exp` decoder.
- Palo Alto PAN-OS set-style and structurally identifiable XML exports.
- Cisco IOS/IOS-XE (combined unless the source itself distinguishes them),
  NX-OS and ASA. Ordinary overlapping switch syntax receives the neutral
  **Network config** label instead of an unsupported vendor guess.

## Recognition and safety boundaries

Recognition examines at most the first 65,536 characters. It uses a strong
anchored format header or multiple independent command/context cues. A filename,
brand mentioned inside a description, or a lone `interface`, `vlan`, or `set`
command is not proof of a vendor. Conflicting vendor evidence stays generic.
Quoted values and banner bodies are excluded from detection. Generic XML does
not become PAN-OS merely because it has a `config` root.

Highlighting is **lexical display**, not a parser, correctness check, security
review, configuration conversion or device action. The editor does not execute
commands, resolve XML entities, fetch remote schemas, decode opaque exports or
send configuration content to an external language service. Existing binary-file
handling is unchanged. Unknown exports remain unsupported/plain rather than
pretending to understand encrypted or proprietary formats.

Command words are colored at command positions, fields at recognized field
positions, and values separately. Comments, quoted text, banners, descriptions,
and sensitive-value tails are opaque to command keyword coloring. This is not
secret redaction: a configuration opened by its user still contains its original
values, so avoid publishing screenshots of real configurations.

Source text, whitespace, BOM, Unicode, CRLF, lone CR, tabs and control characters
are never rewritten by detection/highlighting. XML is escaped before display and
never parsed into a live document. Network syntax does not enable a guessed
formatter or CLI comment-insertion command. Manual editing and saving retain the
editor's existing encoding and line-ending protections.

Syntax painting supports up to **1,000,000 characters for network formats**;
other language grammars keep their existing **250,000-character** cap. Network
painting also has a 100,000-span / 8 MiB escaped-HTML budget. Exceeding a limit
falls back to the **complete**, editable plain source with an explicit notice,
never partial output. Worker operations retain their existing two-second timeout,
and stale/closed paints terminate their worker. Comparison panes have separate fixed highlight
channels, so painting one pane does not cancel the other or the main editor.
Closing a pane cancels only its stream; closing the editor disposes all workers.

## Sources and inference

Recognition rules are our conservative inference from these primary references,
not vendor-certified parsers or a claim that every release/export is supported:

- [HPE Aruba CX VLAN command contexts](https://arubanetworking.hpe.com/techdocs/AOS-CX/10.17/HTML/l2_bridging_8100-83xx-93xx-100xx/Content/Chp_vlans/vlan_cmds/vla-acc.htm)
  and [CX software branding/platform version form](https://arubanetworking.hpe.com/techdocs/AOS-CX/10.16/HTML/fundamentals_6300-6400/Content/SysHW_cmds/sho-ver-10.htm).
- [ArubaOS-Switch VLAN-context port membership](https://arubanetworking.hpe.com/techdocs/AOS-S/16.10/ATMG/WC/content/common%20files/cnf-cha-sta-vla-per-por-set-cli.htm).
- [H3C Comware saved/running configuration examples](https://www.h3c.com/en/d_201905/1177651_294551_0.htm).
- [Fortinet FortiOS CLI reference, command hierarchy/subcommands](https://fortinetweb.s3.amazonaws.com/docs.fortinet.com/v2/attachments/c287b6bf-a995-11e9-81a4-00505692583a/FortiOS-6.0.8-CLI_Reference.pdf)
  (pages 29–35; command structure, not an exhaustive current command inventory).
- [SonicWall CLI text export structure and terminal artifacts](https://www.sonicwall.com/support/knowledge-base/how-to-copy-a-sonicos-cli-configuration-from-the-command-line/kA1VN0000000LAx0AM).
- [Palo Alto set/XML configuration forms](https://knowledgebase.paloaltonetworks.com/KCSArticleDetail?id=kA10g000000ClHoCAK).
- [Cisco IOS-XE VLAN/interface syntax](https://www.cisco.com/c/en/us/td/docs/switches/lan/c9000/lyr2-fwd/vlan/vlan-configuration-guide/configure-vlan-trunks.html),
  [NX-OS feature syntax](https://developer.cisco.com/docs/nx-os/linux-shell-tools/),
  [NX-OS boot syntax](https://www.cisco.com/c/en/us/td/docs/switches/datacenter/nexus3000/sw/command/reference/5_0_3/fund/3k_cmd_ref_fund/3k_cmd_ref_fund_cmds.html),
  and [ASA interface security syntax](https://www.cisco.com/c/en/us/td/docs/security/asa/asa-cli-reference/S/asa-command-ref-S/sa-shov-commands.html).

## Regression coverage

`node --test scripts/network-config-language.test.cjs scripts/editor-languages.test.cjs scripts/editor-language-service.test.cjs`

Fixtures are synthetic. Tests cover each family, ambiguous and conflicting
content, comments/banner/quote negatives, sensitive tails, plain override,
source-text round trips, malicious markup, large malformed input, preserved
programming-language behavior, and independent comparison worker cancellation.
Hosted editor/browser tests additionally verify actual pixels and per-tab/compare
behavior. A local unit pass alone is not a browser or live-device acceptance.
