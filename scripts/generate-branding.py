# Copyright 2026 17Artist. Licensed under the project's LICENSE.
"""Generate the original Armour2BBModel vector mark and Windows icon sizes.

Pillow is needed only when regenerating branding, never by the shipped EXE.
The polygons below are the source of both the SVG and raster exports.
"""

from pathlib import Path

from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parent.parent
SIZES = (16, 24, 32, 48, 64, 128, 256)
BACKGROUND = "#242327"
FACES = (
    # Extruded right and bottom faces.
    ("#63417D", ((304, 104), (324, 124), (422, 418), (402, 398))),
    ("#795395", ((110, 398), (188, 398), (208, 418), (130, 418))),
    ("#63417D", ((324, 398), (402, 398), (422, 418), (344, 418))),
    # Flat-topped, block-cut A silhouette.
    ("#B180D7", ((208, 104), (304, 104), (402, 398), (324, 398),
                 (298, 310), (214, 310), (188, 398), (110, 398))),
    ("#CCA8E9", ((208, 104), (256, 104), (244, 182), (220, 254),
                 (214, 310), (188, 398), (110, 398))),
    ("#9563BC", ((256, 104), (304, 104), (402, 398), (324, 398),
                 (298, 310), (292, 254), (268, 182))),
    # The crossbar and the large, readable counter.
    ("#B991DA", ((220, 254), (292, 254), (298, 310), (214, 310))),
    (BACKGROUND, ((244, 182), (268, 182), (292, 254), (220, 254))),
    ("#DEC6EF", ((208, 104), (304, 104), (284, 128), (226, 128))),
)


def vector() -> str:
    polygons = "\n".join(
        f'  <polygon fill="{color}" points="'
        + " ".join(f"{x},{y}" for x, y in points)
        + '"/>'
        for color, points in FACES
    )
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" '
        'viewBox="0 0 512 512">\n'
        f'  <rect x="8" y="8" width="496" height="496" rx="112" '
        f'fill="{BACKGROUND}" stroke="#4B415B" stroke-width="2"/>\n'
        f'{polygons}\n</svg>\n'
    )


def raster(size: int) -> Image.Image:
    # Draw each size independently with supersampling, preserving clean edges
    # and transparent outer corners rather than rescaling a cached tiny icon.
    scale = size * 4 / 512
    image = Image.new("RGBA", (size * 4, size * 4), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle(
        tuple(round(value * scale) for value in (8, 8, 504, 504)),
        radius=round(112 * scale),
        fill=BACKGROUND,
        outline="#4B415B",
        width=max(1, round(2 * scale)),
    )
    for color, points in FACES:
        draw.polygon([(round(x * scale), round(y * scale)) for x, y in points], fill=color)
    return image.resize((size, size), Image.Resampling.LANCZOS)


def main() -> None:
    branding = ROOT / "branding"
    branding.mkdir(exist_ok=True)
    svg = vector()
    (branding / "armour2bbmodel.svg").write_text(svg, encoding="utf-8")
    (ROOT / "web/static/favicon.svg").write_text(svg, encoding="utf-8")
    raster(512).save(branding / "armour2bbmodel.png", optimize=True)
    frames = [raster(size) for size in SIZES]
    frames[-1].save(
        branding / "armour2bbmodel.ico", format="ICO", sizes=[(size, size) for size in SIZES],
        append_images=frames[:-1],
    )
    with Image.open(branding / "armour2bbmodel.ico") as icon:
        assert icon.ico.sizes() == {(size, size) for size in SIZES}
        for size in SIZES:
            frame = icon.ico.getimage((size, size))
            assert frame.size == (size, size) and frame.mode == "RGBA"
            assert frame.getpixel((0, 0))[3] == 0
    print("BRANDING_EXPORT_PASS: SVG, 512px PNG, ICO " + ", ".join(map(str, SIZES)))


if __name__ == "__main__":
    main()
