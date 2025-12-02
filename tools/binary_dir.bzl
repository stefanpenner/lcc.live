"""Macro to create a directory containing symlinks to multiple binaries for PATH usage."""

def binary_dir(name, binaries, **kwargs):
    """Creates a directory containing symlinks to multiple binaries.
    
    This macro creates:
    - A binary_dir rule that creates symlinks in name_bin/ subdirectory
    - Individual filegroup targets for each binary for easy reference
    
    Args:
        name: Name of the target (creates name + "_bin" directory)
        binaries: List of binary labels (e.g., ["@tools_rsvg_convert//:rsvg-convert", "@tools_magick//:magick"])
        **kwargs: Additional arguments passed to the rule
    """
    # Create the binary_dir rule
    _binary_dir_impl(
        name = name,
        binaries = binaries,
        **kwargs
    )
    
    # Create filegroups for individual binaries so they can be referenced
    for binary in binaries:
        # Extract binary name from label
        binary_name = binary.split(":")[-1]
        native.filegroup(
            name = name + "_" + binary_name,
            srcs = [":" + name],
            output_group = binary_name,
            visibility = ["//visibility:public"],
        )

def _binary_dir_impl_rule(ctx):
    """Implementation for binary_dir rule."""
    # Collect input files and create symlinks
    symlink_outputs = []
    output_groups = {}
    
    for binary in ctx.attr.binaries:
        files = binary.files.to_list()
        if not files:
            fail("Binary {} has no files".format(binary.label))
        
        binary_file = files[0]
        
        # Extract binary name from file basename
        binary_name = binary_file.basename
        
        # Create symlink file in a subdirectory
        symlink_file = ctx.actions.declare_file(ctx.attr.name + "_bin/" + binary_name)
        ctx.actions.symlink(
            output = symlink_file,
            target_file = binary_file,
            is_executable = True,
        )
        symlink_outputs.append(symlink_file)
        # Store by name for output group access
        output_groups[binary_name] = depset([symlink_file])
    
    return [
        DefaultInfo(
            files = depset(symlink_outputs),
        ),
        OutputGroupInfo(**output_groups),
    ]

_binary_dir_impl = rule(
    implementation = _binary_dir_impl_rule,
    attrs = {
        "binaries": attr.label_list(
            mandatory = True,
            doc = "List of binary targets to include in the directory",
            allow_files = True,
        ),
    },
    doc = "Creates a directory containing symlinks to multiple binaries for PATH usage",
)

