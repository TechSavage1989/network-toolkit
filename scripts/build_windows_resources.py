"""Generate a standard Windows COFF resource object before Go links the EXE.

Only Python's standard library is needed. The executable is never patched.
The generated object is included so ordinary source builds require Go only.
"""
from pathlib import Path
import struct
from generate_icon_assets import validate_assets

ROOT = Path(__file__).resolve().parents[1]


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


def version_info():
    fixed = struct.pack('<13I', 0xFEEF04BD, 0x00010000, 0x00010011, 0x00910000, 0x00010011, 0x00910000, 0x3F, 0, 0x40004, 1, 0, 0, 0)
    strings = {
        'FileDescription': 'Network Toolkit - Portable Network Diagnostics',
        'FileVersion': '1.17.145.0',
        'InternalName': 'NetworkToolkit',
        'OriginalFilename': 'Network_Toolkit.exe',
        'ProductName': 'Network Toolkit',
        'ProductVersion': '1.17.145',
    }
    table = version_block('040904B0', children=[version_block(k, utf16(v), text=True) for k, v in strings.items()], text=True)
    string_info = version_block('StringFileInfo', children=[table], text=True)
    var_info = version_block('VarFileInfo', children=[version_block('Translation', struct.pack('<HH', 1033, 1200))], text=True)
    return version_block('VS_VERSION_INFO', fixed, [string_info, var_info])


def main():
    validate_assets(ROOT)
    icon = (ROOT / 'assets/Network_Toolkit_Icon.ico').read_bytes()
    reserved, kind, count = struct.unpack_from('<HHH', icon)
    if reserved or kind != 1 or not 1 <= count <= 30:
        raise ValueError('Invalid ICO')
    icons = {}
    group = bytearray(struct.pack('<HHH', 0, 1, count))
    for i in range(count):
        width, height, colors, res, planes, bits, size, offset = struct.unpack_from('<BBBBHHII', icon, 6 + i * 16)
        data = icon[offset:offset+size]
        if len(data) != size:
            raise ValueError('Truncated ICO entry')
        icons[i+1] = {1033: data}
        group.extend(struct.pack('<BBBBHHIH', width, height, colors, res, planes, bits, size, i+1))
    tree = {3: icons, 14: {1: {1033: bytes(group)}}, 16: {1: {1033: version_info()}}, 24: {1: {1033: (ROOT/'assets/app.manifest').read_bytes()}}}
    section = bytearray()
    leaves = []

    def directory(items):
        align(section)
        at = len(section)
        section.extend(b'\0'*(16+8*len(items)))
        struct.pack_into('<IIHHHH', section, at, 0, 0, 0, 0, 0, len(items))
        for i, (key, value) in enumerate(sorted(items.items())):
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
    output = ROOT / 'cmd/network-toolkit/resource_windows_amd64.syso'
    output.write_bytes(header + sh + section + relocations + symbol + struct.pack('<I', 4))
    print(f'{output.name}: {output.stat().st_size} bytes, {len(leaves)} native resources')


if __name__ == '__main__':
    main()
