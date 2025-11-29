"""Module extension to find host tools for favicon generation."""

def _find_host_tool(repository_ctx, tool_name, paths):
    """Find a tool on the host system."""
    # Try each path with the tool name appended
    for path_dir in paths:
        full_path = path_dir + "/" + tool_name
        path_obj = repository_ctx.path(full_path)
        if path_obj.exists:
            return str(path_obj)
    # Try which as fallback
    result = repository_ctx.execute(["which", tool_name], quiet = True)
    if result.return_code == 0:
        return result.stdout.strip()
    return None

def _host_tools_repo_impl(repository_ctx):
    """Repository rule implementation to find host tools."""
    
    # Get tools and paths from repository_ctx.attr
    tools = repository_ctx.attr.tools  # List of tool names
    default_paths = repository_ctx.attr.default_paths  # List of directories to search
    
    filegroup_srcs = {}
    symlinks = {}
    
    for tool_name in tools:
        tool_binary = _find_host_tool(repository_ctx, tool_name, default_paths)
        
        if not tool_binary:
            fail("{} not found in paths: {}. Please install it.".format(tool_name, ", ".join(default_paths)))
        
        # Create symlink with a unique name
        symlink_name = tool_name.replace("-", "_").replace("/", "_") + "_bin"
        repository_ctx.symlink(tool_binary, symlink_name)
        symlinks[tool_name] = symlink_name
    
    # Create BUILD file with filegroups
    filegroup_content = ""
    for tool_name, symlink_name in symlinks.items():
        filegroup_name = tool_name.replace("-", "_").replace("/", "_")
        filegroup_content += """
filegroup(
    name = "{filegroup_name}",
    srcs = ["{symlink_name}"],
    visibility = ["//visibility:public"],
)
""".format(filegroup_name = filegroup_name, symlink_name = symlink_name)
    
    repository_ctx.file("BUILD.bazel", content = filegroup_content)

# Repository rule that can be used by the module extension
_host_tools_repo = repository_rule(
    implementation = _host_tools_repo_impl,
    local = True,
    attrs = {
        "tools": attr.string_list(mandatory = True, doc = "List of tool names to find"),
        "default_paths": attr.string_list(mandatory = True, doc = "List of directories to search for tools"),
    },
)

def _host_tools_ext_impl(module_ctx):
    """Module extension implementation."""
    for mod in module_ctx.modules:
        for tool_config in mod.tags.host_tool:
            # Get default paths or use sensible defaults
            default_paths = tool_config.default_paths or [
                "/opt/homebrew/bin",
                "/usr/local/bin",
                "/usr/bin",
            ]
            
            # Get tools list
            tools = tool_config.tools or []
            
            # Create the repository using the repository rule
            _host_tools_repo(
                name = tool_config.name,
                tools = tools,
                default_paths = default_paths,
            )

host_tools_ext = module_extension(
    implementation = _host_tools_ext_impl,
    tag_classes = {
        "host_tool": tag_class(attrs = {
            "name": attr.string(mandatory = True),
            "tools": attr.string_list(mandatory = True, doc = "List of tool names to find (e.g., ['rsvg-convert', 'magick'])"),
            "default_paths": attr.string_list(doc = "List of directories to search for tools (defaults to common locations)"),
        }),
    },
)
