"""Generate the standard Windows COFF resource objects before Go links the EXEs.

Only Python's standard library is needed. The executables are never patched.
The generated objects are included so ordinary source builds require Go only.

One object per program (docs/SEPARATE_EXECUTABLES.md): Network_Toolkit.exe in
cmd/network-toolkit, and the separate Text Editor and Terminal programs in
internal/winres/<edition>, which only the build-tagged edition imports. Every
program carries the same icon groups at the same IDs, so the native window code
loads the same icons in each. A separate program adds one named group, APP,
that refers to its own app's icon images: named groups come first in a resource
directory, so it is icon index 0 (Explorer, Task Manager, Start, pins), and the
main program's indexes follow it shifted by one. The main program's object is
unchanged, so every index that Setup's registrations name keeps its icon.
"""
from pathlib import Path
import struct
from generate_icon_assets import validate_assets, validate_document_icon

ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / 'cmd/network-toolkit/resource_windows_amd64.syso'
# Icon groups in resource ID order (1, 2, ...). Windows numbers them from zero
# in the same order, and that number is what shortcuts and registry values such
# as '"Network_Toolkit.exe",1' name: 0 app, 1 Text Editor, 2 Terminal (window
# icons use the IDs, internal/desktop/desktop.go), 3 the text document icon of
# files associated with the Text Editor (Setup only). Released Setups and
# shortcuts already point at these numbers: never reorder or remove a group,
# and append a new one at the end.
ICON_GROUPS = ('Icon', 'Editor', 'Terminal', 'Document')


def align(buffer, boundary=4):
    buffer.extend(b'\0' * (-len(buffer) % boundary))


def utf16(value):
    return (value + '\0').encode('utf-16le')


def version_block(key, value=b'', children=(), text=False):
    block = bytearray(b'\0' * 6 + utf16(key))
    align(block)
    block.extend(value)
    for child in children:
        align(block)
        block.extend(child)
    struct.pack_into('<HHH', block, 0, len(block), len(value) // 2 if text else len(value), int(text))
    return bytes(block)


# (output, FileDescription, OriginalFilename, icon group used as index 0 or None)
PROGRAMS = (
    ('cmd/network-toolkit/resource_windows_amd64.syso', 'Network Toolkit - Network Diagnostics', 'Network_Toolkit.exe', None),
    ('internal/winres/editor/resource_windows_amd64.syso', 'Network Toolkit Text Editor', 'Network_Toolkit_TextEditor.exe', 2),
    ('internal/winres/terminal/resource_windows_amd64.syso', 'Network Toolkit Terminal', 'Network_Toolkit_Terminal.exe', 3),
)
APP_GROUP = 'APP'


def version_info(description=PROGRAMS[0][1], original=PROGRAMS[0][2]):
    fixed = struct.pack('<13I', 0xFEEF04BD, 0x00010000, 0x00010015, 0x002E0000, 0x00010015, 0x002E0000, 0x3F, 0, 0x40004, 1, 0, 0, 0)
    strings = {
        'FileDescription': description,
        'FileVersion': '1.21.46.0',
        'InternalName': 'NetworkToolkit',
        'OriginalFilename': original,
        'ProductName': 'Network Toolkit',
        'ProductVersion': '1.21.46',
    }
    table = version_block('040904B0', children=[version_block(k, utf16(v), text=True) for k, v in strings.items()], text=True)
    string_info = version_block('StringFileInfo', children=[table], text=True)
    var_info = version_block('VarFileInfo', children=[version_block('Translation', struct.pack('<HH', 1033, 1200))], text=True)
    return version_block('VS_VERSION_INFO', fixed, [string_info, var_info])


def resource_object(groups_tree, icons, description, original):
    """The COFF object of one program: icons, groups, version and manifest."""
    tree = {3: icons, 14: groups_tree, 16: {1: {1033: version_info(description, original)}},
            24: {1: {1033: (ROOT/'assets/app.manifest').read_bytes()}}}
    section = bytearray()
    leaves = []

    def directory(items):
        align(section)
        at = len(section)
        named = sorted(key for key in items if isinstance(key, str))
        numbered = sorted(key for key in items if not isinstance(key, str))
        section.extend(b'\0'*(16+8*len(items)))
        struct.pack_into('<IIHHHH', section, at, 0, 0, 0, 0, len(named), len(numbered))
        for i, key in enumerate(named + numbered):
            value = items[key]
            if isinstance(key, str):
                # IMAGE_RESOURCE_DIR_STRING_U: a length and UTF-16 text, at a
                # section offset (no relocation, unlike the data entries).
                align(section, 2)
                name_at = len(section)
                text = key.encode('utf-16le')
                section.extend(struct.pack('<H', len(text) // 2) + text)
                key = name_at | 0x80000000
            if isinstance(value, dict):
                child = directory(value) | 0x80000000
            else:
                align(section)
                child = len(section)
                section.extend(b'\0'*16)
                leaves.append((child, value))
            struct.pack_into('<II', section, at+16+8*i, key, child)
        return at

    directory(tree)
    relocations = bytearray()
    for location, data in leaves:
        align(section)
        offset = len(section)
        section.extend(data)
        struct.pack_into('<IIII', section, location, offset, len(data), 0, 0)
        relocations.extend(struct.pack('<IIH', location, 0, 3))
    align(section)
    raw_offset = 60
    reloc_offset = raw_offset + len(section)
    symbols_offset = reloc_offset + len(relocations)
    header = struct.pack('<HHIIIHH', 0x8664, 1, 0, symbols_offset, 1, 0, 0)
    sh = struct.pack('<8sIIIIIIHHI', b'.rsrc\0\0\0', 0, 0, len(section), raw_offset, reloc_offset, 0, len(leaves), 0, 0x40300040)
    symbol = struct.pack('<8sIhHBB', b'.rsrc\0\0\0', 0, 1, 0, 3, 0)
    return header + sh + section + relocations + symbol + struct.pack('<I', 4), len(leaves)


def icon_tables():
    """The icon images and the numbered groups shared by every program."""
    icons = {}
    groups = {}
    for group_id, name in enumerate(ICON_GROUPS, 1):
        icon = (ROOT / f'assets/Network_Toolkit_{name}.ico').read_bytes()
        reserved, kind, count = struct.unpack_from('<HHH', icon)
        if reserved or kind != 1 or not 1 <= count <= 30:
            raise ValueError(f'Invalid {name} ICO')
        group = bytearray(struct.pack('<HHH', 0, 1, count))
        for i in range(count):
            width, height, colors, res, planes, bits, size, offset = struct.unpack_from('<BBBBHHII', icon, 6 + i * 16)
            data = icon[offset:offset+size]
            if len(data) != size:
                raise ValueError(f'Truncated {name} ICO entry')
            icon_id = len(icons) + 1
            icons[icon_id] = {1033: data}
            group.extend(struct.pack('<BBBBHHIH', width, height, colors, res, planes, bits, size, icon_id))
        groups[group_id] = {1033: bytes(group)}
    return icons, groups


def program_resources():
    """{output path: object bytes} for every program."""
    validate_assets(ROOT)
    validate_document_icon(ROOT)
    icons, groups = icon_tables()
    result = {}
    for output, description, original, app_group in PROGRAMS:
        tree = dict(groups)
        if app_group is not None:
            tree[APP_GROUP] = groups[app_group]
        result[output] = resource_object(tree, icons, description, original)
    return result


def main():
    for output, (data, leaves) in program_resources().items():
        path = ROOT / output
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        print(f'{output}: {path.stat().st_size} bytes, {leaves} native resources')


if __name__ == '__main__':
    main()
