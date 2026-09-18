#!/usr/bin/env python3
"""Build host-specific, runtime-free plugin archives using the standard library."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import zipfile

ROOT = Path(__file__).resolve().parents[1]
HOSTS = {"chatgpt": ".codex-plugin", "claude": ".claude-plugin"}

# opencode is packaged differently because it works differently: it has no
# plugin manifest and no marketplace, it discovers skills as directories, and
# it takes its MCP server from the user's own config file. So its archive
# carries the same skills plus a config fragment to merge and the steps to
# merge it — there is nothing for a manifest to declare.
OPENCODE_DIR = ".opencode-plugin"
OPENCODE_FILES = ("opencode.json", "INSTALL.md")


def manifest_files(root):
    version = (root / "VERSION").read_text().strip()
    if not re.fullmatch(r"\d+\.\d+\.\d+", version):
        raise ValueError("VERSION must be a release version")
    manifests = {}
    for host, folder in HOSTS.items():
        manifest = json.loads((root / folder / "plugin.json").read_text())
        if manifest.get("name") != "plasma-plugin" or manifest.get("version") != version:
            raise ValueError(f"{host} name/version does not match VERSION")
        if any(key in manifest for key in ("mcpServers", "hooks", "apps")):
            raise ValueError("Source manifests must not include runtime or deployment bindings")
        manifests[host] = manifest
    return version, manifests


def build(root=ROOT, output=None, app_id=None):
    version, manifests = manifest_files(root)
    if app_id is not None and not re.fullmatch(r"(?:asdk_app_|connector_|templated_apps_)[A-Za-z0-9_-]+", app_id):
        raise ValueError("Use a real registered app ID, not a plugin ID or URL")
    tag = os.environ.get("GITHUB_REF_NAME", "")
    if tag and tag != f"v{version}":
        raise ValueError(f"Release tag must match VERSION: v{version}")
    output = Path(output) if output else root / "dist" / f"v{version}"
    output.mkdir(parents=True, exist_ok=True)
    skills = sorted((root / "skills").rglob("*.md"))
    if not skills:
        raise ValueError("No skills found")
    payloads = {}
    for host, folder in HOSTS.items():
        manifest = manifests[host]
        files = {str(p.relative_to(root)): p.read_bytes() for p in skills}
        # Keep developer docs, tooling, hooks and executables out of both packages.
        if host == "chatgpt" and app_id:
            manifest["apps"] = "./.app.json"
            files[".app.json"] = encode({"apps": {"plasma": {"id": app_id, "required": True}}})
        files[f"{folder}/plugin.json"] = encode(manifest)
        suffix = "-linked" if host == "chatgpt" and app_id else ""
        payloads[f"{host}{suffix}"] = files

    # The opencode archive ships the config fragment at the top level rather
    # than under a dotted directory: the user opens it and merges it by hand,
    # so it has to be visible.
    opencode = {str(p.relative_to(root)): p.read_bytes() for p in skills}
    for name in OPENCODE_FILES:
        source = root / OPENCODE_DIR / name
        if not source.is_file():
            raise ValueError(f"opencode package is missing {OPENCODE_DIR}/{name}")
        opencode[name] = source.read_bytes()
    payloads["opencode"] = opencode

    archives = []
    for host, files in payloads.items():
        archive = output / f"plasma-plugin_{version}_{host}.zip"
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as zf:
            for name, content in sorted(files.items()):
                # Fixed timestamps and modes make repeated packages reproducible.
                info = zipfile.ZipInfo(f"plasma-plugin/{name}", (2020, 1, 1, 0, 0, 0))
                info.external_attr = 0o100644 << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                zf.writestr(info, content)
        archives.append(archive)
    checksum_name = "checksums-linked.txt" if app_id else "checksums.txt"
    (output / checksum_name).write_text("".join(
        f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n" for p in archives
    ))
    return archives


def encode(value):
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--app-id", help="Existing ChatGPT app ID for a workspace-linked package; not a public MCP submission")
    args = parser.parse_args()
    try:
        for artifact in build(output=args.output, app_id=args.app_id):
            print(artifact)
    except ValueError as error:
        parser.error(str(error))
