"""Derive safe runtime SVG, PNG and ICO from the editable canonical vector icon.

Regeneration needs Inkscape on PATH; ordinary builds use checked-in outputs and
only Python's standard library. The editable source is never passed directly to an SVG renderer.
No DTD/entity resolution, external resource fetching or script execution occurs.

`--document` regenerates only the text document icon (the paper sheet Windows
shows for files associated with the Text Editor) and never touches the
published app icon.
"""
from pathlib import Path
import argparse
import hashlib
import json
import os
import re
import struct
import subprocess
import tempfile
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
ORIGINAL = 'assets/source/Network_Toolkit_Icon.original.svg'
SVG = 'assets/Network_Toolkit_Icon.svg'
PNG = 'assets/Network_Toolkit_Icon.png'
ICO = 'assets/Network_Toolkit_Icon.ico'
MANIFEST = 'assets/Network_Toolkit_Icon.manifest.json'
SIZES = (16, 20, 24, 32, 40, 48, 64, 128, 256)
# The icon of files associated with the Text Editor: the editor icon's paper
# sheet without its tile. The small sizes have their own drawings on the pixel
# grid, so their edges stay sharp; every other size is rendered from the 256 px
# drawing.
DOCUMENT = 'assets/source/Network_Toolkit_Document.svg'
DOCUMENT_HINTS = {size: f'assets/source/Network_Toolkit_Document.{size}.svg' for size in (16, 20, 24, 32)}
DOCUMENT_ICO = 'assets/Network_Toolkit_Document.ico'
DOCUMENT_MANIFEST = 'assets/Network_Toolkit_Document.manifest.json'
DOCUMENT_SIZES = (16, 20, 24, 32, 40, 48, 64, 96, 128, 256)
NS = 'http://www.w3.org/2000/svg'
ENTITIES = {
    'ns_extend': 'http://ns.adobe.com/Extensibility/1.0/',
    'ns_ai': 'http://ns.adobe.com/AdobeIllustrator/10.0/',
    'ns_graphs': 'http://ns.adobe.com/Graphs/1.0/',
    'ns_vars': 'http://ns.adobe.com/Variables/1.0/',
    'ns_imrep': 'http://ns.adobe.com/ImageReplacement/1.0/',
    'ns_sfw': 'http://ns.adobe.com/SaveForWeb/1.0/',
    'ns_custom': 'http://ns.adobe.com/GenericCustomNamespace/1.0/',
    'ns_adobe_xpath': 'http://ns.adobe.com/XPath/1.0/',
}
TAGS = {'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polyline', 'polygon'}
PAINT = {'fill', 'stroke', 'stroke-width', 'stroke-linejoin', 'stroke-linecap',
         'fill-rule', 'clip-rule', 'opacity', 'fill-opacity', 'stroke-opacity',
         'stroke-miterlimit', 'stroke-dasharray', 'stroke-dashoffset'}
ATTRS = PAINT | {'id', 'd', 'x', 'y', 'x1', 'y1', 'x2', 'y2', 'width', 'height',
                 'rx', 'ry', 'cx', 'cy', 'r', 'points', 'transform'}
ET.register_namespace('', NS)


def presentation_value(value):
    """Accept only unescaped presentation values, never CSS resource syntax.

    SVG presentation attributes use CSS syntax too. Reject escapes, comments and
    controls before trimming so resource spellings cannot evade the same checks
    used for class and inline styles. This is intentionally not a general CSS
    sanitizer: unsupported syntax in future originals must be reviewed explicitly.
    """
    if re.search(r'[\\\x00-\x1f\x7f-\x9f<>&@{}]|/\*|\*/|url\s*\(|expression\s*\(', value, re.I):
        raise ValueError('Unsupported or unsafe SVG presentation value')
    value = value.strip()
    if not value:
        raise ValueError('Empty SVG presentation value')
    return value


def declarations(text):
    result = {}
    for part in text.split(';'):
        if not part.strip():
            continue
        key, separator, value = part.partition(':')
        key = key.strip()
        if not separator or key not in PAINT:
            raise ValueError('Unsupported or unsafe SVG presentation property')
        result[key] = presentation_value(value)
    return result


def safe_source(raw):
    """Remove a recognized namespace-only internal subset before XML parsing."""
    if len(raw) > 2 << 20:
        raise ValueError('Icon source exceeds 2 MiB')
    text = raw.decode('utf-8')
    if re.search(r'<\?xml-stylesheet', text, re.I):
        raise ValueError('External stylesheet processing instructions are forbidden')
    pattern = r'<!DOCTYPE\s+svg\s+PUBLIC\s+"-//W3C//DTD SVG 1\.1//EN"\s+"http://www\.w3\.org/Graphics/SVG/1\.1/DTD/svg11\.dtd"\s*\[([\s\S]*?)\]>'
    match = re.search(pattern, text)
    if match:
        subset = match.group(1)
        found = re.findall(r'<!ENTITY\s+(\w+)\s+"([^"]*)"\s*>', subset)
        remainder = re.sub(r'<!ENTITY\s+(\w+)\s+"([^"]*)"\s*>', '', subset)
        if remainder.strip() or len({name for name, _ in found}) != len(found):
            raise ValueError('Unsupported SVG entity declaration')
        for name, value in found:
            if ENTITIES.get(name) != value:
                raise ValueError('Only recognized literal Adobe namespace entities are allowed')
        text = text[:match.start()] + text[match.end():]
        for name, value in found:
            text = text.replace('&' + name + ';', value)
    if re.search(r'<!\s*(?:DOCTYPE|ENTITY)', text, re.I):
        raise ValueError('DTD and entity resolution is forbidden')
    root = ET.fromstring(text)
    if root.tag != '{' + NS + '}svg' or root.get('viewBox') != '0 0 256 256':
        raise ValueError('Expected a 256 by 256 SVG icon')
    for key, value in root.attrib.items():
        if key not in {'version', 'id', 'x', 'y', 'width', 'height', 'viewBox', 'style', '{http://www.w3.org/XML/1998/namespace}space'}:
            raise ValueError('Unsupported SVG root attribute: ' + key)
        if key == 'style' and re.sub(r'\s', '', value).rstrip(';') != 'enable-background:new00256256':
            raise ValueError('Unsupported root presentation style')
        if key in {'x', 'y'} and value not in {'0', '0px'}:
            raise ValueError('Unexpected SVG root offset')
    for node in root.iter():
        if node.tag == '{' + NS + '}script':
            raise ValueError('SVG scripts are forbidden')
        if node.tag == '{' + NS + '}foreignObject':
            if node.attrib != {'requiredExtensions': ENTITIES['ns_ai'], 'x': '0', 'y': '0', 'width': '1', 'height': '1'}:
                raise ValueError('Only the non-rendered Illustrator editor reference may be omitted')
            if len(node) != 1 or node[0].tag != '{' + ENTITIES['ns_ai'] + '}aipgfRef' or node[0].get('{http://www.w3.org/1999/xlink}href') != '#adobe_illustrator_pgf':
                raise ValueError('Unexpected Illustrator editor reference')
        if node.tag == '{' + NS + '}style' and node not in list(root):
            raise ValueError('Nested SVG stylesheets are unsupported')
        for key, value in node.attrib.items():
            local = key.rsplit('}', 1)[-1]
            if local.lower().startswith('on'):
                raise ValueError('SVG event handlers are forbidden')
            if local in {'href', 'src'} and not value.startswith('#'):
                raise ValueError('External SVG references are forbidden')
            if re.search(r'url\s*\(|@import|javascript:', value, re.I):
                raise ValueError('External SVG resources are forbidden')
    return root


def runtime_svg(raw, inline_styles=True):
    """Keep the exact vector geometry while excluding all editor-only content."""
    source = safe_source(raw)
    styles = {}
    style_nodes = []
    for node in source.findall('{' + NS + '}style'):
        if len(node):
            raise ValueError('SVG stylesheet elements cannot contain child elements')
        css = node.text or ''
        rules = re.findall(r'\.([A-Za-z_][\w-]*)\s*\{([^{}]*)\}', css)
        if re.sub(r'\.([A-Za-z_][\w-]*)\s*\{([^{}]*)\}', '', css).strip():
            raise ValueError('Unsupported SVG stylesheet')
        for name, body in rules:
            if name in styles:
                raise ValueError('Duplicate SVG class')
            styles[name] = declarations(body)
        # The reference renderer also receives only reconstructed SVG nodes,
        # never copied attributes or foreign descendants from the source.
        clean_style = ET.Element('{' + NS + '}style', {'type': 'text/css'})
        clean_style.text = css
        style_nodes.append(clean_style)
    output = ET.Element('{' + NS + '}svg', {'width': '256', 'height': '256', 'viewBox': '0 0 256 256'})
    if not inline_styles:
        output.extend(style_nodes)

    def display(node):
        if not node.tag.startswith('{' + NS + '}'):
            return None  # Illustrator embedded editable document is not runtime data.
        tag = node.tag.rsplit('}', 1)[-1]
        if tag in {'style', 'metadata', 'foreignObject'}:
            return None
        if tag not in TAGS:
            raise ValueError('Unsupported SVG drawing element: ' + tag)
        attrs = {}
        for name in (node.get('class') or '').split():
            if name not in styles:
                raise ValueError('Unknown SVG class')
            if inline_styles:
                attrs.update(styles[name])
        for key, value in node.attrib.items():
            if key.startswith('{'):
                continue  # Editable labels remain in the preserved source only.
            if key == 'class':
                if not inline_styles:
                    attrs[key] = value
            elif key == 'style':
                attrs.update(declarations(value))
            elif key in PAINT:
                attrs[key] = presentation_value(value)
            elif key in ATTRS:
                attrs[key] = value
            else:
                raise ValueError('Unsupported SVG drawing attribute: ' + key)
        result = ET.Element('{' + NS + '}' + tag, attrs)
        for child in node:
            clean = display(child)
            if clean is not None:
                result.append(clean)
        return result

    for child in source:
        if child.tag == '{' + NS + '}switch':
            choices = [n for n in child if n.tag != '{' + NS + '}foreignObject']
            if len(choices) != 1 or choices[0].tag != '{' + NS + '}g':
                raise ValueError('Expected exactly one SVG fallback drawing')
            child = choices[0]
        clean = display(child)
        if clean is not None:
            output.append(clean)
    if not output.findall('.//{' + NS + '}path'):
        raise ValueError('Icon has no vector drawing')
    ET.indent(output, space='  ')
    return ET.tostring(output, encoding='utf-8', xml_declaration=True) + b'\n'


def digest(data):
    return hashlib.sha256(data).hexdigest()


def validate_assets(root=ROOT):
    manifest = json.loads((root / MANIFEST).read_text())
    if manifest.get('version') != 1 or manifest.get('sizes') != list(SIZES):
        raise ValueError('Unsupported icon asset manifest')
    expected = {ORIGINAL, SVG, PNG, ICO}
    if set(manifest.get('sha256', {})) != expected:
        raise ValueError('Icon asset manifest does not cover the exact expected files')
    for path in sorted(expected):
        if digest((root / path).read_bytes()) != manifest['sha256'][path]:
            raise ValueError('Icon asset changed; regenerate with scripts/generate_icon_assets.py: ' + path)
    if runtime_svg((root / ORIGINAL).read_bytes()) != (root / SVG).read_bytes():
        raise ValueError('Runtime SVG does not match the preserved canonical original')


def document_sources():
    """Each source of the document icon, in manifest order."""
    return [DOCUMENT] + [DOCUMENT_HINTS[size] for size in sorted(DOCUMENT_HINTS)]


def ico_entries(raw):
    """(size, PNG bytes) of each image of an ICO, checking its exact layout."""
    if len(raw) < 6:
        raise ValueError('Truncated ICO')
    reserved, kind, count = struct.unpack_from('<HHH', raw)
    if reserved or kind != 1 or not 1 <= count <= 30 or len(raw) < 6 + 16 * count:
        raise ValueError('Invalid ICO header')
    offset = 6 + 16 * count
    entries = []
    for index in range(count):
        width, height, colors, res, planes, bits, size, start = struct.unpack_from('<BBBBHHII', raw, 6 + 16 * index)
        png = raw[start:start + size]
        if start != offset or len(png) != size:
            raise ValueError('ICO images are not stored in order without gaps')
        if png[:8] != b'\x89PNG\r\n\x1a\n' or png[24:26] != b'\x08\x06':
            raise ValueError('ICO image is not an 8-bit RGBA PNG')
        pixels = struct.unpack_from('>II', png, 16)
        if (width or 256, height or 256) != pixels or pixels[0] != pixels[1] or (colors, res, planes, bits) != (0, 0, 1, 32):
            raise ValueError('ICO directory entry does not describe its image')
        entries.append((pixels[0], png))
        offset += size
    if offset != len(raw):
        raise ValueError('ICO has trailing data')
    return entries


def validate_document_icon(root=ROOT):
    manifest = json.loads((root / DOCUMENT_MANIFEST).read_text())
    if manifest.get('version') != 1 or manifest.get('sizes') != list(DOCUMENT_SIZES):
        raise ValueError('Unsupported document icon manifest')
    expected = set(document_sources()) | {DOCUMENT_ICO}
    if set(manifest.get('sha256', {})) != expected:
        raise ValueError('Document icon manifest does not cover the exact expected files')
    for path in sorted(expected):
        if digest((root / path).read_bytes()) != manifest['sha256'][path]:
            raise ValueError('Document icon changed; regenerate with scripts/generate_icon_assets.py --document: ' + path)
    for path in document_sources():
        runtime_svg((root / path).read_bytes())
    if [size for size, _ in ico_entries((root / DOCUMENT_ICO).read_bytes())] != list(DOCUMENT_SIZES):
        raise ValueError('Document icon does not hold the expected sizes')


def render_png(inkscape, svg, size, temp):
    """Render reconstructed (safe) SVG bytes to one square RGBA PNG."""
    safe = temp / f'safe-{size}.svg'
    safe.write_bytes(svg)
    path = temp / (str(size) + '.png')
    subprocess.run([inkscape, str(safe), '--export-type=png', '--export-filename=' + str(path), '--export-width=' + str(size), '--export-height=' + str(size)], check=True, env={**os.environ, 'INKSCAPE_PROFILE_DIR': str(temp / 'inkscape-profile')})
    data = path.read_bytes()
    if data[:8] != b'\x89PNG\r\n\x1a\n' or struct.unpack('>II', data[16:24]) != (size, size):
        raise ValueError('Renderer returned an invalid PNG')
    return data


def pack_ico(sizes, images):
    icon = bytearray(struct.pack('<HHH', 0, 1, len(sizes)))
    offset = 6 + 16 * len(sizes)
    for size, data in zip(sizes, images):
        icon.extend(struct.pack('<BBBBHHII', size % 256, size % 256, 0, 0, 1, 32, len(data), offset))
        offset += len(data)
    for data in images:
        icon.extend(data)
    return bytes(icon)


def generate_document_icon(inkscape):
    sources = {path: runtime_svg((ROOT / path).read_bytes()) for path in document_sources()}
    with tempfile.TemporaryDirectory(prefix='network-toolkit-document-icon-') as temp:
        temp = Path(temp)
        images = [render_png(inkscape, sources[DOCUMENT_HINTS.get(size, DOCUMENT)], size, temp) for size in DOCUMENT_SIZES]
    (ROOT / DOCUMENT_ICO).write_bytes(pack_ico(DOCUMENT_SIZES, images))
    paths = document_sources() + [DOCUMENT_ICO]
    manifest = {'version': 1, 'sizes': list(DOCUMENT_SIZES), 'sha256': {path: digest((ROOT / path).read_bytes()) for path in paths}}
    (ROOT / DOCUMENT_MANIFEST).write_text(json.dumps(manifest, indent=2) + '\n')
    validate_document_icon()
    print(f'Generated the {len(DOCUMENT_SIZES)}-size text document ICO')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--inkscape', default='inkscape')
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--document', action='store_true', help='regenerate only the text document icon')
    args = parser.parse_args()
    if args.check:
        validate_assets()
        validate_document_icon()
        print('Canonical icon source and runtime assets verified')
        return
    if args.document:
        generate_document_icon(args.inkscape)
        return
    clean = runtime_svg((ROOT / ORIGINAL).read_bytes())
    with tempfile.TemporaryDirectory(prefix='network-toolkit-icons-') as temp:
        temp = Path(temp)
        images = [render_png(args.inkscape, clean, size, temp) for size in SIZES]
        icon = pack_ico(SIZES, images)
        (ROOT / SVG).write_bytes(clean)
        (ROOT / PNG).write_bytes(images[-1])
        (ROOT / ICO).write_bytes(icon)
    manifest = {'version': 1, 'sizes': list(SIZES), 'sha256': {path: digest((ROOT / path).read_bytes()) for path in (ORIGINAL, SVG, PNG, ICO)}}
    (ROOT / MANIFEST).write_text(json.dumps(manifest, indent=2) + '\n')
    validate_assets()
    print('Generated safe SVG, 256px PNG and nine-size ICO from the preserved original')


if __name__ == '__main__':
    main()
