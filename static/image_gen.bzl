"""Macro for generating images from SVG files using rsvg-convert and magick."""

def image_from_svg(name, script, svg_src, output, **kwargs):
    """Generate an image from an SVG file using a script.
    
    Args:
        name: Name of the genrule target
        script: Label of the script to run (e.g., //scripts:generate-favicon)
        svg_src: Label of the SVG source file
        output: Output filename
        **kwargs: Additional arguments passed to genrule
    """
    native.genrule(
        name = name,
        srcs = [svg_src],
        outs = [output],
        cmd = """
            export PATH="$$(dirname $(execpath @tools_rsvg_convert//:rsvg-convert)):$$(dirname $(execpath @tools_magick//:magick)):$$PATH" && \
            $(execpath {script}) \
                $(location {svg_src}) \
                $(location :{output})
        """.format(
            script = script,
            svg_src = svg_src,
            output = output,
        ),
        tools = [
            script,
            "@tools_magick//:magick",
            "@tools_rsvg_convert//:rsvg-convert",
        ],
        visibility = ["//visibility:public"],
        **kwargs
    )

