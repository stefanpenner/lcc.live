"""Repository rule to download and extract binaries from archives.

Inspired by http_archive, but handles platform-specific URLs and binary extraction.
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

def _normalize_platform_key(os_type, arch):
    """Normalize platform key for URL/sha256 dict lookup."""
    return "{}_{}".format(os_type, arch)

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

def _find_local_binary(repository_ctx, binary_name):
    """Try to find a binary on the local system as a fallback."""
    which_result = repository_ctx.execute(["which", binary_name], quiet = True)
    if which_result.return_code == 0:
        return which_result.stdout.strip()
    return None

def _http_binary_impl(repository_ctx):
    """Repository rule implementation to download and extract a binary."""
    os_type, arch = _detect_os_arch(repository_ctx)
    platform_key = _normalize_platform_key(os_type, arch)
    
    # Get URLs and sha256 - either from direct attributes or config file
    urls = []
    sha256 = None
    
    if repository_ctx.attr.config_file:
        # Load from config file
        config = json.decode(repository_ctx.read(repository_ctx.attr.config_file))
        tool_configs = config.get("tools") or fail("Config must contain 'tools' key")
        tool_name = repository_ctx.attr.tool_name or repository_ctx.name
        tool_config = tool_configs.get(tool_name) or fail("No config for tool: {}".format(tool_name))
        
        binary_name = tool_config.get("binary_name", tool_name)
        urls = [
            url for url in tool_config.get("urls", {}).get(os_type, {}).get(arch, [])
            if url and "placeholder" not in url.lower()
        ]
        sha256 = tool_config.get("sha256", {}).get(os_type, {}).get(arch) or None
    else:
        # Use direct attributes (inspired by http_archive)
        binary_name = repository_ctx.attr.binary_name or repository_ctx.name.replace("_", "-")
        
        # Get URLs for current platform
        platform_urls = repository_ctx.attr.urls.get(platform_key) if repository_ctx.attr.urls else []
        if not platform_urls:
            # Try legacy format: nested dict
            platform_urls = repository_ctx.attr.urls.get(os_type, {}).get(arch, []) if repository_ctx.attr.urls else []
        
        urls = [url for url in platform_urls if url]
        
        # Get SHA256 for current platform
        if repository_ctx.attr.sha256:
            sha256 = repository_ctx.attr.sha256.get(platform_key) or repository_ctx.attr.sha256.get(os_type, {}).get(arch) or None
    
    # If no URLs, try local binary
    if not urls:
        local_binary = _find_local_binary(repository_ctx, binary_name)
        if local_binary:
            repository_ctx.symlink(local_binary, binary_name)
            repository_ctx.file("BUILD.bazel", content = 'exports_files(["{}"], visibility = ["//visibility:public"])\n'.format(binary_name))
            return
        fail("No URLs configured for {} on {}, and tool not found locally. Provide urls or install the tool.".format(binary_name, platform_key))
    
    # Download and extract
    extract_dir = "extracted"
    repository_ctx.execute(["mkdir", "-p", extract_dir])
    
    binary_path = None
    for url in urls:
        binary_path = _download_and_extract_archive(repository_ctx, url, sha256, extract_dir, binary_name)
        if binary_path:
            break
    
    if not binary_path:
        fail("Failed to download {} from any URL".format(binary_name))
    
    # Create symlink with actual binary name and BUILD file
    repository_ctx.symlink(binary_path, binary_name)
    repository_ctx.file("BUILD.bazel", content = 'exports_files(["{}"], visibility = ["//visibility:public"])\n'.format(binary_name))

# Repository rule inspired by http_archive
http_binary = repository_rule(
    implementation = _http_binary_impl,
    attrs = {
        # Direct URL specification (like http_archive.urls)
        "urls": attr.string_list_dict(
            doc = "Dictionary mapping platform keys (e.g., 'linux_amd64') to lists of URLs. Can also use nested format: {'linux': {'amd64': [...]}}",
        ),
        # SHA256 checksums (like http_archive.sha256)
        "sha256": attr.string_dict(
            doc = "Dictionary mapping platform keys (e.g., 'linux_amd64') to SHA256 checksums. Can also use nested format: {'linux': {'amd64': '...'}}",
        ),
        # Binary name to extract
        "binary_name": attr.string(
            doc = "Name of the binary file to extract from the archive. Defaults to repository name with underscores replaced by hyphens.",
        ),
        # Alternative: config file (for multiple tools)
        "config_file": attr.label(
            doc = "Path to JSON configuration file (alternative to direct urls/sha256 attributes)",
            allow_single_file = True,
        ),
        "tool_name": attr.string(
            doc = "Tool name in config file (only used with config_file). Defaults to repository name.",
        ),
    },
)

# Module extension to load multiple binaries from config file
def _http_binaries_ext_impl(module_ctx):
    """Module extension implementation that creates http_binary repos from config."""
    for mod in module_ctx.modules:
        for repo_config in mod.tags.binary:
            # Get tools list - use provided tools or read from config file
            tools = repo_config.tools or []
            
            # If tools not specified, read config file to get all tools
            if not tools:
                # Read the config file to get all tool names
                config_content = module_ctx.read(repo_config.config_file)
                config = json.decode(config_content)
                tool_configs = config.get("tools") or fail("Config must contain 'tools' key")
                tools = tool_configs.keys()
            
            # Create individual http_binary repos for specified tools
            name_prefix = repo_config.name_prefix or ""
            for tool_name in tools:
                repo_name = tool_name.replace("-", "_").replace("/", "_")
                if name_prefix:
                    repo_name = "{}_{}".format(name_prefix, repo_name)
                
                http_binary(
                    name = repo_name,
                    config_file = repo_config.config_file,
                    tool_name = tool_name,
                )



http_binaries_ext = module_extension(
    implementation = _http_binaries_ext_impl,
    tag_classes = {
        "binary": tag_class(attrs = {
            "name_prefix": attr.string(
                doc = "Prefix for repository names (defaults to empty)",
                default = "",
            ),
            "tools": attr.string_list(
                doc = "List of tool names to download (defaults to all tools in config)",
            ),
            "config_file": attr.label(
                mandatory = True,
                doc = "Path to JSON configuration file",
                allow_single_file = True,
            ),
        }),
    },
)
