# Terminal display names — 1.17.128

Names are display metadata, never connection identity or proof that a command
succeeded. The precedence is **manual name > recognized hostname > endpoint**.
A name appears above the smaller endpoint; an identical endpoint is not repeated.
Saved addresses, ports, users and credential links are unchanged.

Use **Use device name / IP** to clear a manual display override. This keeps the
saved profile's identifying text, but disables that text as the display override;
the learned name or endpoint is shown instead. Saving or renaming explicitly
enables the supplied manual name again. Display-name revisions prevent stale
window snapshots from restoring an older override after rename or clear.

## Recognition boundaries

Recognition uses one stable, normal-buffer final cursor line with no pending
input/output. ANSI color is handled by xterm rendering, not by parsing raw escape
sequences. Wrapped output, typed commands, generic prompts, control characters,
IP-like dotted numeric tokens and unrecognized structures do not supply a name.
Captured hostnames are bounded to 100 ASCII letters, digits, dots, underscores
and hyphens; numeric-only device names are allowed. Manual names may contain
ordinary spaces and Unicode within their existing bounded display policy.

Prompts are customizable and can omit or spoof hostnames. It is impossible to
infer every arbitrary prompt reliably. Unsupported or ambiguous forms retain a
previous reliable name or use the endpoint; a manual name works independently.
This recognizer does **not** expand the separate prompt-paced sending/readiness
grammar. A recognized display name does not authorize automatic command sending.

## Implemented synthetic format coverage

Examples are synthetic. Coverage below means parser fixtures (and selected actual
xterm/browser fixtures), not live-device or every-firmware certification. Final
release acceptance is recorded in [the handoff](RELEASE128_HANDOFF.md).

| Family | Representative form | Extracted name / boundary |
|---|---|---|
| Numeric devices; Cisco IOS/XE/NX, Arista EOS, Dell OS10, Ruckus, Aruba CX/S | `7300#`, `EDGE-01(config-if)#`, `EDGE-01(conf-if-eth1/1/7)#` | Prefix; bounded known config/interface mode discarded. Recovery and role-only prompts rejected. |
| Junos, PAN-OS, SonicWall operational | `operator@EDGE-01>` | `EDGE-01`; preceding edit/role lines are not concatenated. |
| Linux / EdgeOS | `operator@LINUX-01:~/work$` | `LINUX-01`, not username or directory. Paths containing ambiguous spaces/substitutions are outside this grammar. |
| fish / zsh common forms | `operator@LINUX-01 ~>`, `operator@LINUX-01 ~ %`, `LINUX-01%` | Host only; arbitrary custom prompt functions are not decoded. |
| RouterOS | `[operator@EDGE-01] /ip/route<SAFE>` | Bracketed host; menu path and safe-mode marker excluded. |
| ESXi / Check Point Expert | `[root@ESXI-02:~]`, `[Expert@FW-01:0]#` | Host, not directory or virtual-system index. |
| PowerShell remoting | `[SERVER-01]: PS C:\tools>` | Remote host; local PowerShell/CMD path-only prompts rejected. |
| Cisco IOS XR | `RP/0/RP0/CPU0:EDGE-01(config)#` | Host after documented hardware prefix, including numeric slot form. Admin VM prompts rejected. |
| Nokia classic / MD-CLI | `*A:EDGE-01>config>router#`, `A:operator@EDGE-01#` | Host, excluding slot, unsaved marker, user and hierarchy. |
| NetApp ONTAP / SP | `cluster1::*>`, `cluster1::cluster>`, `SP cluster1-01>` | Cluster/host; generic LOADER prompt rejected. Inline timestamp customizations are outside this grammar. |
| Aruba controller / Instant AP | `(EDGE-01) [mynode] (config) #`, `(EDGE-01)#` | Parenthesized host; bounded safe-name policy narrower than vendor's customizable names. Not a claim of Instant On CLI support. |
| FortiGate / SonicWall config | `FW-01 (global) #`, `config(FW-01)#` | Host; hostless nested config prompts preserve prior identity. |
| F5 tmsh | `operator@BIGIP-01(Active)(/Common)(tmos.net)#` | Host; bounded recognized status/partition/module contexts, not all custom partitions/prompts. |
| Extreme EXOS | `* Slot-1 Stack.3 #`, `X440G2-24t-10G4.4 #` | Strip command counter only with explicit slot/model shape or known prior host. Ambiguous dotted hostnames are not blindly truncated. |
| HPE Comware / Huawei VRP user view | `<EDGE-01>` | Entire bounded inner host. |
| Comware / VRP system and subviews | `[EDGE-01-vlan100]`, `[*EDGE-01-aaa]` | Requires a previously known matching host; hyphen boundary is ambiguous otherwise. |
| Cisco ASA slash contexts | `EDGE-01/admin/pri/act(config)#` | Requires a known matching host. Configurable field order can omit hostname; first slash field is not assumed to be a host. |

Negative fixtures include banners/prose, appended commands, bare sigils,
`console>`, `auditor>`, `LOADER-A>`, `switch(boot)#`, shell-version prompts,
`[edit interfaces]`, local paths, pagers and unsafe/overlong input. Known names
survive unsupported hostless modes, reload and scoped session transfers.

## Primary-source survey

Research date: 2026-10-30. These references establish representative structures,
not universal support for every vendor customization.

- [Cisco IOS-XE names and custom prompts](https://www.cisco.com/c/en/us/td/docs/routers/ios/config/17-x/syst-mgmt/b-system-management/m_bsm-basic-sys-manage-xe.html), [IOS XR CLI](https://www.cisco.com/c/en/us/td/docs/routers/crs/software/crs_r4-1/getting_started/configuration/guide/gs41crs/gs41cnov.html), [ASA configurable prompt fields](https://www.cisco.com/c/en/us/td/docs/security/asa/asa912/configuration/general/asa-912-general-config/admin-management.html).
- [Aruba AOS-S prompt explanations](https://arubanetworking.hpe.com/techdocs/AOS-S/16.10/FCI/content/fci/switch%20prompt%20explanation.htm), [controller prompts](https://arubanetworking.hpe.com/techdocs/CLI-Bank/Content/aos8/prompt.htm), [Instant hostname limits](https://arubanetworking.hpe.com/techdocs/CLI-Bank/Content/instant/hostname.htm), [HPE Comware](https://support.hpe.com/hpesc/public/api/document/c05028262?v=1769501383000), [Huawei commit-state wrappers](https://info.support.huawei.com/network/ptmngsys/Web/tsrev_ne/en/content/ne/10_edesk_vrp_new_or_modified_configuration_cannot_take_effect_v8/edesk_vrp_new_or_modified_configuration_cannot_take_effect_v8_edesk002.html).
- [Junos CLI](https://www.juniper.net/documentation/us/en/software/junos/cli/topics/topic-map/cli-configuration.html), [Junos custom prompts](https://www.juniper.net/documentation/us/en/software/junos/cli-reference/topics/ref/command/set-cli-prompt.html), [RouterOS configuration management](https://manual.mikrotik.com/docs/getting-started/configuration-management/), [SonicWall configuration examples](https://www.sonicwall.com/support/knowledge-base/how-to-copy-a-sonicos-cli-configuration-from-the-command-line/kA1VN0000000LAx0AM).
- [EdgeOS root shell](https://help.uisp.com/hc/en-us/articles/22591204009367-EdgeRouter-Root-User-Account), [ESXi shell](https://knowledge.broadcom.com/external/article/419885/unable-to-ssh-to-esxi-host.html), [PowerShell prompts/remoting](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_prompts), [FortiGate examples](https://docs.fortinet.com/document/fortigate/7.6.6/administration-guide/80552/tpm-support-for-fortigate-vm).
- [Bash prompt customization](https://www.gnu.org/software/bash/manual/html_node/Controlling-the-Prompt.html), [zsh prompt expansion](https://zsh.sourceforge.io/Doc/Release/Prompt-Expansion.html), [fish interactive prompts](https://fishshell.com/docs/4.0/interactive.html), [fish hostname helper](https://fishshell.com/docs/current/cmds/prompt_hostname.html), [PAN-OS CLI modes](https://docs.paloaltonetworks.com/ngfw/pan-os-cli-quick-start/get-started-with-the-cli/change-cli-modes), [CMD custom prompts](https://learn.microsoft.com/windows-server/administration/windows-commands/prompt).
- [Nokia classic prompt](https://infocenter.nokia.com/public/7210SAS229R1A/topic/com.nokia.DEK_Basic_System_Guide/the_cli_command-ai9j4okjwf.html), [Nokia MD navigation](https://documentation.nokia.com/sr/24-3/7x50-shared/md-cli-user/navigate.html), [F5 tmsh platform guide](https://techdocs.f5.com/content/kb/en-us/products/big-ip_ltm/manuals/product/pg_1600/_jcr_content/pdfAttach/download/file.res/Platform_Guide__1600.pdf), [NetApp privilege prompts](https://docs.netapp.com/us-en/ontap-cli/set.html), [NetApp SP and loader](https://kb.netapp.com/on-prem/ontap/OHW/OHW-KBs/How_to_perform_graceful_shutdown_and_power_up_of_all_ONTAP_nodes_in_a_cluster).
- [Arista CLI](https://www.arista.com/en/um-eos/eos-command-line-interface-cli), [Dell OS10 hostname](https://www.dell.com/support/manuals/en-uk/dell-emc-smartfabric-os10/smartfabric-os-user-guide-10-5-3/hostname?guid=guid-1300ccc6-8203-4329-9ef6-c46b5c21e1e0&lang=en-us), [Ruckus FastIron guide](https://docs-be.commscope.com/bundle/fastiron-08092-managementguide/raw/resource/enus/fastiron-08092-managementguide.pdf), [Extreme stack prompt](https://documentation.extremenetworks.com/exos_16.1/GUID-804835FA-F750-484E-835E-C549D337ACDA.shtml), [Extreme VPEX](https://documentation.extremenetworks.com/exos_commands_31.6/GUID-973BBA8D-B19D-4F20-B41E-CF03167EA67F.shtml).

Fixture references: `scripts/terminal-prompt-labels128-parser.test.cjs`,
`scripts/terminal-prompt-labels127.browser.cjs`, and the retained observer,
manual-name, session-transfer and native WebView2 tests. Research breadth does
not replace exact final-source execution of these gates.
