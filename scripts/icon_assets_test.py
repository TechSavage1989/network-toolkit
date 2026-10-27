"""Regression guards for canonical icon provenance and safe runtime assets."""
from pathlib import Path
import json
import shutil
import struct
import tempfile
import unittest
import xml.etree.ElementTree as ET

import generate_icon_assets as icons


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

    def test_external_entities_scripts_and_resources_fail_closed(self):
        original = (icons.ROOT / icons.ORIGINAL).read_bytes()
        simple = b'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><path d="M0 0h1"/></svg>'
        invalid = [
            original.replace(b'http://ns.adobe.com/Extensibility/1.0/', b'file:///unreadable-entity-target'),
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
