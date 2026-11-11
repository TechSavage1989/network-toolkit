"""Regression guards for canonical icon provenance and safe runtime assets."""
from pathlib import Path
import json
import shutil
import struct
import tempfile
import unittest
import xml.etree.ElementTree as ET
import zlib

import generate_icon_assets as icons

PUBLISHED_ICONS = {
    'Editor': '2bad5e7500e491c949538789a7f2d831e05a78f3f68b08dc7fea086f6586e524',
    'Terminal': '5ce0600802717940569802c6f0f5dccc57e39d88d2b939012353cf7860ce4149',
}


def rgba_rows(png):
    """Decode a non-interlaced 8-bit RGBA PNG into rows of bytes."""
    width, height, depth, color, _, _, interlace = struct.unpack_from('>IIBBBBB', png, 16)
    if (depth, color, interlace) != (8, 6, 0):
        raise ValueError('Expected a non-interlaced 8-bit RGBA PNG')
    data, at = b'', 8
    while at < len(png):
        length, kind = struct.unpack_from('>I4s', png, at)
        if kind == b'IDAT':
            data += png[at + 8:at + 8 + length]
        at += 12 + length
    raw, stride, rows, previous = zlib.decompress(data), width * 4, [], bytes(width * 4)
    for y in range(height):
        kind, line = raw[y * (stride + 1)], bytearray(raw[y * (stride + 1) + 1:(y + 1) * (stride + 1)])
        for i in range(stride):
            left = line[i - 4] if i >= 4 else 0
            up, corner = previous[i], previous[i - 4] if i >= 4 else 0
            if kind == 1:
                line[i] = (line[i] + left) & 255
            elif kind == 2:
                line[i] = (line[i] + up) & 255
            elif kind == 3:
                line[i] = (line[i] + (left + up) // 2) & 255
            elif kind == 4:
                p = left + up - corner
                pa, pb, pc = abs(p - left), abs(p - up), abs(p - corner)
                line[i] = (line[i] + (left if pa <= pb and pa <= pc else up if pb <= pc else corner)) & 255
        rows.append(bytes(line))
        previous = line
    return rows


class IconAssetsTests(unittest.TestCase):
    def test_manifest_covers_canonical_source_and_all_generated_assets(self):
        icons.validate_assets()
        manifest = json.loads((icons.ROOT / icons.MANIFEST).read_text())
        self.assertEqual(set(manifest['sha256']), {icons.ORIGINAL, icons.SVG, icons.PNG, icons.ICO})

    def test_runtime_keeps_original_vector_geometry_without_editor_content(self):
        original = (icons.ROOT / icons.ORIGINAL).read_bytes()
        source = icons.safe_source(original)
        runtime = icons.runtime_svg(original)
        self.assertEqual(runtime, (icons.ROOT / icons.SVG).read_bytes())
        clean = ET.fromstring(runtime)
        ns = '{' + icons.NS + '}'
        self.assertEqual([p.get('d') for p in source.iter(ns + 'path')], [p.get('d') for p in clean.iter(ns + 'path')])
        for forbidden in (b'<!DOCTYPE', b'<!ENTITY', b'foreignObject', b'aipgf', b'<script', b'<style', b'&ns_'):
            self.assertNotIn(forbidden, runtime)
        self.assertEqual(clean.attrib, {'width': '256', 'height': '256', 'viewBox': '0 0 256 256'})

    def test_ico_contains_all_nine_png_sizes_with_exact_payload_bounds(self):
        raw = (icons.ROOT / icons.ICO).read_bytes()
        self.assertEqual(struct.unpack_from('<HHH', raw), (0, 1, len(icons.SIZES)))
        expected_offset = 6 + 16 * len(icons.SIZES)
        resource = (icons.ROOT / 'cmd/network-toolkit/resource_windows_amd64.syso').read_bytes()
        for index, expected_size in enumerate(icons.SIZES):
            width, height, colors, reserved, planes, bits, count, offset = struct.unpack_from('<BBBBHHII', raw, 6 + 16 * index)
            self.assertEqual((width or 256, height or 256, colors, reserved, planes, bits), (expected_size, expected_size, 0, 0, 1, 32))
            self.assertEqual(offset, expected_offset)
            png = raw[offset:offset + count]
            self.assertEqual(len(png), count)
            self.assertEqual(png[:8], b'\x89PNG\r\n\x1a\n')
            self.assertEqual(struct.unpack_from('>II', png, 16), (expected_size, expected_size))
            self.assertEqual(png[24:26], b'\x08\x06')  # 8-bit RGBA, including transparency.
            self.assertIn(png, resource, 'Compile-time native resource is stale')
            expected_offset += count
            if expected_size == 256:
                self.assertEqual(png, (icons.ROOT / icons.PNG).read_bytes())
        self.assertEqual(expected_offset, len(raw))

    def test_build_rejects_stale_source_or_generated_bytes(self):
        for target in (icons.ORIGINAL, icons.SVG, icons.PNG, icons.ICO):
            with self.subTest(target=target), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                for name in (icons.ORIGINAL, icons.SVG, icons.PNG, icons.ICO, icons.MANIFEST):
                    destination = root / name
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(icons.ROOT / name, destination)
                with (root / target).open('ab') as output:
                    output.write(b'changed')
                with self.assertRaisesRegex(ValueError, 'Icon asset changed'):
                    icons.validate_assets(root)

    def test_approved_icon_is_editable_and_uses_only_solid_colors(self):
        original = (icons.ROOT / icons.ORIGINAL).read_bytes()
        source = icons.safe_source(original)
        ns = '{' + icons.NS + '}'
        groups = {g.get('id') for g in source.findall(ns + 'g')}
        self.assertTrue({'background', 'terminal-window', 'terminal-controls',
                         'transfer-arrows', 'network-topology', 'network-node-cutouts'} <= groups)
        palette = {'#071F43', '#08B9F2', '#FFFFFF', '#FF8B17'}
        for node in source.iter():
            for name in ('fill', 'stroke'):
                paint = node.get(name)
                if paint and paint != 'none':
                    self.assertIn(paint, palette)
            self.assertNotIn(node.tag.rsplit('}', 1)[-1],
                             {'linearGradient', 'radialGradient', 'filter', 'image', 'pattern'})
        rendered = icons.runtime_svg(original)
        self.assertNotIn(b'gradient', rendered.lower())
        self.assertNotIn(b'url(', rendered.lower())

    def test_document_icon_manifest_covers_every_source_and_the_ico(self):
        icons.validate_document_icon()
        manifest = json.loads((icons.ROOT / icons.DOCUMENT_MANIFEST).read_text())
        self.assertEqual(set(manifest['sha256']), set(icons.document_sources()) | {icons.DOCUMENT_ICO})
        for path in icons.document_sources():
            # Committed SVG keeps LF line endings (see AGENTS.md).
            self.assertNotIn(b'\r', (icons.ROOT / path).read_bytes(), path)

    def test_document_icon_rejects_stale_sources_or_ico(self):
        files = icons.document_sources() + [icons.DOCUMENT_ICO, icons.DOCUMENT_MANIFEST]
        for target in icons.document_sources() + [icons.DOCUMENT_ICO]:
            with self.subTest(target=target), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                for name in files:
                    destination = root / name
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(icons.ROOT / name, destination)
                with (root / target).open('ab') as output:
                    output.write(b'changed')
                with self.assertRaisesRegex(ValueError, 'Document icon changed'):
                    icons.validate_document_icon(root)

    def test_document_icon_is_the_paper_sheet_on_a_transparent_background(self):
        entries = icons.ico_entries((icons.ROOT / icons.DOCUMENT_ICO).read_bytes())
        self.assertEqual([size for size, _ in entries], list(icons.DOCUMENT_SIZES))
        palette = {'#071F43', '#FFF', '#73DDFF'}
        ns = '{' + icons.NS + '}'
        for path in icons.document_sources():
            with self.subTest(path=path):
                source = icons.safe_source((icons.ROOT / path).read_bytes())
                for node in source.iter():
                    # No tile: nothing fills the canvas behind the page.
                    self.assertNotEqual(node.tag, ns + 'rect')
                    for name in ('fill', 'stroke'):
                        paint = node.get(name)
                        if paint and paint != 'none':
                            self.assertIn(paint, palette)
        # The page is drawn from the editor icon's own sheet.
        editor = (icons.ROOT / 'assets/source/Network_Toolkit_Editor.svg').read_text()
        document = icons.safe_source((icons.ROOT / icons.DOCUMENT).read_bytes())
        for path in document.iter(ns + 'path'):
            self.assertIn(path.get('d'), editor)
        # Every image keeps its corners transparent, small sizes included, and
        # the small hand-drawn sizes have hard (fully opaque) page edges.
        for size, png in entries:
            with self.subTest(size=size):
                pixels = rgba_rows(png)
                for x, y in ((0, 0), (size - 1, 0), (0, size - 1), (size - 1, size - 1)):
                    self.assertEqual(pixels[y][4 * x + 3], 0, (x, y))
                alpha = {row[4 * x + 3] for row in pixels for x in range(size)}
                self.assertIn(255, alpha)
                if size in icons.DOCUMENT_HINTS:
                    middle = pixels[size // 2]
                    edges = [middle[4 * x + 3] for x in range(size)]
                    self.assertEqual(set(edges), {0, 255}, 'A blurred page edge at the middle row')

    def test_published_icons_keep_their_bytes(self):
        # Released Setups and shortcuts use these files' groups; regenerating
        # the document icon must never change them. (The app icon is pinned by
        # its manifest.)
        for name in ('Editor', 'Terminal'):
            raw = (icons.ROOT / f'assets/Network_Toolkit_{name}.ico').read_bytes()
            self.assertEqual(icons.digest(raw), PUBLISHED_ICONS[name], name)

    def test_external_entities_scripts_and_resources_fail_closed(self):
        original = (icons.ROOT / icons.ORIGINAL).read_bytes()
        simple = b'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><path d="M0 0h1"/></svg>'
        invalid = [
            b'<!DOCTYPE svg [<!ENTITY leak SYSTEM "file:///unreadable-entity-target">]>' + simple,
            b'<!DOCTYPE svg SYSTEM "file:///unreadable-dtd-target">' + simple,
            simple.replace(b'<path', b'<script/> <path'),
            simple.replace(b'<path', b'<path onload="unsafe()"'),
            simple.replace(b'<path', b'<image href="https://example.invalid/image.png"/> <path'),
            simple.replace(b'<path', b'<path fill="url(https://example.invalid/paint)"'),
            b'<?xml-stylesheet href="https://example.invalid/style.css"?>' + simple,
            simple.replace(b'<path', b'<filter/> <path'),
        ]
        for raw in invalid:
            with self.subTest(raw=raw[:100]), self.assertRaises((ValueError, ET.ParseError)):
                icons.runtime_svg(raw)

    def test_presentation_resources_cannot_hide_in_css_escapes_comments_or_controls(self):
        simple = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256">{}</svg>'
        values = [
            r'u\72l(https://example.invalid/paint)',
            r'\75rl(https://example.invalid/paint)',
            'u/**/rl(https://example.invalid/paint)',
            '#123456/*comment*/',
            '#123456\x7f',
            '#123456\x85',
            '#123456&#9;',
            '#123456&#10;',
        ]
        for value in values:
            for property_name in ('fill', 'stroke'):
                drawings = [
                    f'<style>.test{{{property_name}:{value}}}</style><path class="test" d="M0 0h1"/>',
                    f'<path style="{property_name}:{value}" d="M0 0h1"/>',
                    f'<path {property_name}="{value}" d="M0 0h1"/>',
                ]
                for drawing in drawings:
                    for inline_styles in (True, False):
                        with self.subTest(value=value, drawing=drawing, inline_styles=inline_styles), self.assertRaises(ValueError):
                            icons.runtime_svg(simple.format(drawing).encode(), inline_styles=inline_styles)

    def test_foreign_namespace_branches_and_attributes_never_reach_runtime(self):
        simple = '<svg xmlns="http://www.w3.org/2000/svg" xmlns:editor="urn:editor" viewBox="0 0 256 256">{}</svg>'
        drawing = '<g><path fill="#123456" d="M0 0h1"/></g>'
        foreign = r'<editor:metadata editor:fill="u\72l(https://example.invalid/dropped)"><path fill="u\72l(https://example.invalid/dropped)" d="M9 9h9"/>RESOURCE_SENTINEL</editor:metadata>'
        variants = [
            foreign + drawing,
            drawing.replace('<g>', '<g>' + foreign),
            drawing.replace('<path ', '<path editor:fill="RESOURCE_SENTINEL" '),
            '<metadata>' + foreign + '</metadata>' + drawing,
        ]
        for inline_styles in (True, False):
            expected = icons.runtime_svg(simple.format(drawing).encode(), inline_styles=inline_styles)
            for variant in variants:
                with self.subTest(variant=variant, inline_styles=inline_styles):
                    self.assertEqual(icons.runtime_svg(simple.format(variant).encode(), inline_styles=inline_styles), expected)
            # Even the optional safe reference must not copy foreign stylesheet
            # children or their tails into renderer input.
            nested_style = '<style>.test{fill:#123456}<editor:metadata/>.extra{fill:RESOURCE_SENTINEL}</style>'
            with self.assertRaisesRegex(ValueError, 'cannot contain child elements'):
                icons.runtime_svg(simple.format(nested_style + drawing).encode(), inline_styles=inline_styles)


if __name__ == '__main__':
    unittest.main()
