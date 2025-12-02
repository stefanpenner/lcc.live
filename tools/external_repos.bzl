"""Module extension to download binaries for favicon generation.

All binaries are downloaded directly via HTTP and stored in Bazel's repository directory.
"""

def _detect_os_arch(repository_ctx):
    """Detect the operating system and architecture."""
    os_name = repository_ctx.os.name.lower()
    arch = repository_ctx.os.arch
    
    # Normalize architecture names (Bazel uses aarch64, but config uses arm64)
    if arch == "aarch64":
        arch = "arm64"
    
    if os_name.startswith("linux"):
        os_type = "linux"
    elif os_name.startswith("mac"):
        os_type = "macos"
    else:
        os_type = "unknown"
    
    return (os_type, arch)

def _download_and_extract_archive(repository_ctx, url, sha256, extract_dir, binary_name):
    """Download an archive file and extract the binary.
    
    All operations happen in Bazel's repository directory (managed by Bazel).
    """
    # Download the archive
    archive_path = extract_dir + "/archive"
    download_kwargs = {
        "url": url,
        "output": archive_path,
    }
    if sha256:
        download_kwargs["sha256"] = sha256
    repository_ctx.download(**download_kwargs)
    
    # Determine archive type and extract
    archive_file = repository_ctx.path(archive_path)
    
    # Extract based on file extension
    extract_result = None
    if url.endswith(".deb"):
        # Extract .deb: ar extracts data.tar.*, then tar extracts files
        if repository_ctx.execute(["ar", "x", str(archive_file)], quiet = True, working_directory = extract_dir).return_code != 0:
            return None
        data_tar_result = repository_ctx.execute(["find", extract_dir, "-name", "data.tar.*", "-type", "f"], quiet = True)
        if data_tar_result.return_code == 0 and data_tar_result.stdout.strip():
            data_tar = data_tar_result.stdout.strip().split("\n")[0]
            extract_result = repository_ctx.execute(["tar", "-xf", data_tar, "-C", extract_dir], quiet = True)
        else:
            return None
    elif url.endswith(".tar.gz") or url.endswith(".tgz"):
        extract_result = repository_ctx.execute(["tar", "-xzf", str(archive_file), "-C", extract_dir], quiet = True)
    elif url.endswith(".zip"):
        extract_result = repository_ctx.execute(["unzip", "-q", str(archive_file), "-d", extract_dir], quiet = True)
    else:
        return None
    
    if not extract_result or extract_result.return_code != 0:
        return None
    
    # Find the binary in the extracted archive
    find_result = repository_ctx.execute([
        "find",
        extract_dir,
        "-name",
        binary_name,
        "-type",
        "f",
    ], quiet = True)
    
    if find_result.return_code == 0 and find_result.stdout.strip():
        binary_path = find_result.stdout.strip().split("\n")[0]
        path_obj = repository_ctx.path(binary_path)
        if path_obj.exists:
            # Make it executable
            repository_ctx.execute(["chmod", "+x", binary_path], quiet = True)
            return str(path_obj)
    
    return None

def _find_local_binary(repository_ctx, tool_name):
    """Try to find a binary on the local system as a fallback."""
    which_result = repository_ctx.execute(["which", tool_name], quiet = True)
    if which_result.return_code == 0:
        return which_result.stdout.strip()
    return None

def _get_binary(repository_ctx, tool_name, tool_config):
    """Download and extract a binary directly via HTTP.
    
    Falls back to local binary if URLs are not configured.
    """
    os_type, arch = _detect_os_arch(repository_ctx)
    binary_name = tool_config.get("binary_name", tool_name)
    
    # Get download URLs for this OS/arch, filter out placeholders
    urls = [
        url for url in tool_config.get("urls", {}).get(os_type, {}).get(arch, [])
        if url and "placeholder" not in url.lower()
    ]
    
    # Get SHA256 checksum (handle null/empty)
    sha256 = tool_config.get("sha256", {}).get(os_type, {}).get(arch) or None
    
    # If no URLs, try local binary
    if not urls:
        local_binary = _find_local_binary(repository_ctx, binary_name)
        if local_binary:
            return local_binary
        fail("No URLs configured for {} on {}/{}, and tool not found locally. Update tools/external_repos.json or install the tool.".format(tool_name, os_type, arch))
    
    # Download and extract
    extract_dir = "extracted_" + tool_name.replace("-", "_")
    repository_ctx.execute(["mkdir", "-p", extract_dir])
    
    for url in urls:
        binary_path = _download_and_extract_archive(repository_ctx, url, sha256, extract_dir, binary_name)
        if binary_path:
            return binary_path
    
    fail("Failed to download {} from any URL".format(tool_name))

def _external_repos_repo_impl(repository_ctx):
    """Repository rule implementation to download binaries."""
    config = json.decode(repository_ctx.read(repository_ctx.attr.config_file))
    tool_configs = config.get("tools") or fail("Config must contain 'tools' key")
    
    # Use specified tools or all tools from config
    tools = repository_ctx.attr.tools or tool_configs.keys()
    
    symlinks = {}
    for tool_name in tools:
        tool_config = tool_configs.get(tool_name) or fail("No config for tool: {}".format(tool_name))
        binary_path = _get_binary(repository_ctx, tool_name, tool_config)
        symlink_name = tool_name.replace("-", "_").replace("/", "_") + "_bin"
        repository_ctx.symlink(binary_path, symlink_name)
        symlinks[tool_name] = symlink_name
    
    # Generate BUILD file
    filegroups = "\n".join([
        'filegroup(name = "{}", srcs = ["{}"], visibility = ["//visibility:public"])'.format(
            tool_name.replace("-", "_").replace("/", "_"),
            symlink_name
        )
        for tool_name, symlink_name in symlinks.items()
    ])
    repository_ctx.file("BUILD.bazel", content = filegroups + "\n")

# Repository rule that can be used by the module extension
_external_repos_repo = repository_rule(
    implementation = _external_repos_repo_impl,
    local = True,
    attrs = {
        "tools": attr.string_list(doc = "List of tool names to download (defaults to all tools in config)"),
        "config_file": attr.label(mandatory = True, doc = "Path to JSON configuration file", allow_single_file = True),
    },
)

def _external_repos_ext_impl(module_ctx):
    """Module extension implementation."""
    for mod in module_ctx.modules:
        for repo_config in mod.tags.external_repo:
            _external_repos_repo(
                name = repo_config.name,
                tools = repo_config.tools or [],
                config_file = repo_config.config_file,
            )

external_repos_ext = module_extension(
    implementation = _external_repos_ext_impl,
    tag_classes = {
        "external_repo": tag_class(attrs = {
            "name": attr.string(mandatory = True),
            "tools": attr.string_list(doc = "List of tool names to download (defaults to all tools in config)"),
            "config_file": attr.label(mandatory = True, doc = "Path to JSON configuration file", allow_single_file = True),
        }),
    },
)

