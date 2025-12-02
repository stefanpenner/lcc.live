"""Helper functions for creating shell scripts with shared runfiles initialization."""

def _runfiles_init_content():
    """Returns the runfiles initialization code."""
    return """# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
  source "$0.runfiles/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -d' ' -f2- -d' ')" 2>/dev/null ||
  {
    echo >&2 "ERROR: cannot find $f"
    exit 1
  }
f=
set -e
# --- end runfiles.bash initialization v3 ---"""

def script_with_runfiles(name, script_content, **kwargs):
    """Creates a shell script with runfiles initialization prepended.
    
    Args:
        name: Name of the script file to generate
        script_content: The main script content (without shebang or runfiles init)
        **kwargs: Additional arguments passed to the generated file
    """
    full_script = "#!/bin/bash\nset -euo pipefail\n\n"
    full_script += _runfiles_init_content()
    full_script += "\n\n"
    full_script += script_content
    
    native.genrule(
        name = name + "_gen",
        outs = [name],
        cmd = "cat > $@ <<'EOF'\n" + full_script + "\nEOF",
        **kwargs
    )



