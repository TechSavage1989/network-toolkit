# Network Toolkit icon

The icon represents three main app capabilities: terminal access, two-way file transfer, and connected network devices.

The editable vector source is `assets/source/Network_Toolkit_Icon.original.svg`. It uses named paths and groups without an embedded bitmap.

## Flat colors
- Navy background: #071F43
- Cyan network and terminal controls: #08B9F2
- White terminal and receive arrow: #FFFFFF
- Orange send arrow: #FF8B17

No gradients, shadows, filters, external image references or SVG resources.

## Regeneration
Run `python3 scripts/generate_icon_assets.py` with Inkscape installed, then `python3 scripts/build_windows_resources.py`. Run the icon and release guards before merging.
Windows ICO includes 16, 20, 24, 32, 40, 48, 64, 128, and 256 pixels. Embed the ICO at compile time, never patch the finished executable.

## Text document icon
Files whose type opens in the Text Editor show `assets/Network_Toolkit_Document.ico`: the page of the Text Editor icon (white sheet, #73DDFF fold, three #071F43 lines) with a thin #071F43 outline, enlarged, on a transparent background and without the tile. The 256 px drawing is `assets/source/Network_Toolkit_Document.svg`; 16, 20, 24 and 32 px have their own drawings on the pixel grid (`Network_Toolkit_Document.<size>.svg`) so their edges stay sharp. Regenerate with `python3 scripts/generate_icon_assets.py --document` (Inkscape), then `python3 scripts/build_windows_resources.py`. It is icon group index 3 of the program, after the app, Text Editor and Terminal groups.
