"""The Windows resource object holds every icon group in its fixed order."""
import struct
import unittest

import build_windows_resources as resources
import generate_icon_assets as icons

RT_ICON, RT_GROUP_ICON = 3, 14


def read_resources(data):
    """{type: {id: bytes}} from the single .rsrc section of the COFF object."""
    machine, sections = struct.unpack_from('<HH', data)
    assert (machine, sections) == (0x8664, 1)
    name, _, _, size, start = struct.unpack_from('<8sIIII', data, 20)
    assert name == b'.rsrc\0\0\0'
    section = data[start:start + size]

    def directory(at):
        named, numbered = struct.unpack_from('<HH', section, at + 12)
        assert named == 0, 'Only numbered resources are expected'
        for index in range(numbered):
            key, child = struct.unpack_from('<II', section, at + 16 + 8 * index)
            yield key, child

    tree = {}
    for kind, types in directory(0):
        tree[kind] = {}
        for ident, languages in directory(types & 0x7FFFFFFF):
            (language, leaf), = directory(languages & 0x7FFFFFFF)
            assert language == 1033 and not leaf & 0x80000000
            offset, length = struct.unpack_from('<II', section, leaf)
            tree[kind][ident] = section[offset:offset + length]
    return tree


def ico_images(raw):
    """The images of an ICO file in directory order (any planes value)."""
    _, _, count = struct.unpack_from('<HHH', raw)
    images = []
    for index in range(count):
        width, height, colors, reserved, planes, bits, size, offset = struct.unpack_from('<BBBBHHII', raw, 6 + 16 * index)
        images.append(((width, height, colors, reserved, planes, bits, size), raw[offset:offset + size]))
    return images


class WindowsResourceTests(unittest.TestCase):
    def setUp(self):
        self.data = resources.OUTPUT.read_bytes()
        self.tree = read_resources(self.data)

    def test_committed_object_matches_the_committed_assets(self):
        built, _ = resources.program_resources()[str(resources.OUTPUT.relative_to(resources.ROOT))]
        self.assertEqual(self.data, built, 'Run python3 scripts/build_windows_resources.py')

    def test_icon_groups_keep_released_indexes_and_append_the_document_icon(self):
        # Zero-based indexes named by released Setups, shortcuts and window code.
        self.assertEqual(resources.ICON_GROUPS[:3], ('Icon', 'Editor', 'Terminal'))
        self.assertEqual(resources.ICON_GROUPS.index('Document'), 3)
        self.assertEqual(resources.ICON_GROUPS[-1], 'Document')
        self.assertEqual(sorted(self.tree[RT_GROUP_ICON]), list(range(1, len(resources.ICON_GROUPS) + 1)))

    def test_each_group_holds_exactly_its_own_icon_file(self):
        next_id = 1
        for group_id, name in enumerate(resources.ICON_GROUPS, 1):
            with self.subTest(group=name):
                group = self.tree[RT_GROUP_ICON][group_id]
                images = ico_images((icons.ROOT / f'assets/Network_Toolkit_{name}.ico').read_bytes())
                self.assertEqual(struct.unpack_from('<HHH', group), (0, 1, len(images)))
                for index, (entry, png) in enumerate(images):
                    fields = struct.unpack_from('<BBBBHHIH', group, 6 + 14 * index)
                    self.assertEqual(fields[:7], entry)
                    # Icon IDs are numbered across the groups in order, so the
                    # IDs of earlier groups never change when a group is added.
                    self.assertEqual(fields[7], next_id)
                    self.assertEqual(self.tree[RT_ICON][next_id], png)
                    next_id += 1
        self.assertEqual(sorted(self.tree[RT_ICON]), list(range(1, next_id)))

    def test_document_group_is_the_document_icon_in_every_size(self):
        group = self.tree[RT_GROUP_ICON][resources.ICON_GROUPS.index('Document') + 1]
        count = struct.unpack_from('<H', group, 4)[0]
        sizes = [struct.unpack_from('<B', group, 6 + 14 * index)[0] or 256 for index in range(count)]
        self.assertEqual(sizes, list(icons.DOCUMENT_SIZES))
        editor = self.tree[RT_GROUP_ICON][resources.ICON_GROUPS.index('Editor') + 1]
        self.assertNotEqual(group, editor)


if __name__ == '__main__':
    unittest.main()
