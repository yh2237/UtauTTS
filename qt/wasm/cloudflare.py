#!/usr/bin/env python3
"""PagesとR2の配布資産を組み立て、アップロードと公開検証を行う。"""

import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import mimetypes
import os
from pathlib import Path
import re
import shutil
import time
from urllib.parse import quote, urlsplit
from urllib.request import Request, urlopen


HOST_FILES = (
    "config.js", "asset-paths.js", "bootstrap.js", "engine-loader.js",
    "engine-worker.js", "utautts.js", "qtloader.js",
)
PAGE_LIMIT = 25 * 1024 * 1024
IMMUTABLE_CACHE = "public, max-age=31536000, immutable"


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def content_type(path):
    overrides = {".wasm": "application/wasm", ".js": "application/javascript",
                 ".json": "application/json", ".wav": "audio/wav"}
    return overrides.get(path.suffix.lower(), mimetypes.guess_type(path.name)[0] or "application/octet-stream")


def checked_file(root, relative):
    path = (root / relative).resolve()
    if not path.is_relative_to(root.resolve()) or not path.is_file():
        raise ValueError(f"missing or invalid release file: {relative}")
    return path


def deployment_plan(event, ref, preview_name, revision, run_id, attempt, version):
    version_pattern = r"v[0-9]+\.[0-9]+\.[0-9]+"
    if not re.fullmatch(version_pattern, version):
        raise ValueError("appinfo version must have the form vX.X.X")
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("revision must be a full git SHA")
    if not run_id.isdigit() or not attempt.isdigit():
        raise ValueError("run id and attempt must be numeric")
    tag = ref.removeprefix("refs/tags/") if ref.startswith("refs/tags/") else None
    if event == "push" and tag:
        if not re.fullmatch(version_pattern, tag):
            raise ValueError("release tag must have the form vX.X.X")
        if tag != version:
            raise ValueError(f"tag {tag} differs from appinfo version {version}")
        production = True
        label = tag
    elif event == "workflow_dispatch":
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,39}", preview_name):
            raise ValueError("preview name must be 1-40 lowercase letters, digits or hyphens")
        production = False
        label = preview_name
    else:
        raise ValueError("deployments require a release tag or a manual preview")
    branch = "main" if production else "preview-" + label
    deployment_id = f"{label}-{revision[:12]}-{run_id}-{attempt}"
    return {"channel": "releases" if production else "previews", "branch": branch,
            "deployment_id": deployment_id, "version": version, "revision": revision}


def asset_files(source):
    # テスト出力や追加ZIPが混入しないよう、配布対象を明示する。
    files = {
        "utautts.wasm",
        "renderer/utautts-world-phrase/renderer.json",
        *["web/dist/" + name for name in (
            "utautts.wasm", "wasm_exec.js", "fs-shim.js", "openjtalk-bridge.js",
            "world-bridge.js", "openjtalk/utautts-openjtalk.js",
            "openjtalk/utautts-openjtalk.wasm", "world/utautts-world.js",
            "world/utautts-world.wasm", "openjtalk/dict-manifest.json",
            "models/manifest.json", "voice/manifest.json",
        )],
    }
    for name in read_json(checked_file(source, "web/dist/openjtalk/dict-manifest.json"))["files"]:
        files.add("web/dist/openjtalk/dict/" + name)
    for name in read_json(checked_file(source, "web/dist/models/manifest.json"))["models"]:
        files.add("web/dist/models/" + name)
    for name in read_json(checked_file(source, "web/dist/voice/manifest.json"))["files"]:
        files.add("web/dist/voice/" + name)
    return sorted(files)


def assemble(source, output, public_url, deployment_id, version, revision, channel):
    source, output = source.resolve(), output.resolve()
    if source == output or source in output.parents or output in source.parents:
        raise ValueError("source and output must be separate directories")
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9._-]{0,159}", deployment_id):
        raise ValueError("deployment id must be a single URL-safe path component")
    parsed = urlsplit(public_url)
    if parsed.scheme not in ("http", "https") or not parsed.netloc or parsed.query or parsed.fragment:
        raise ValueError("R2 public URL must be an absolute HTTP(S) directory URL")
    if parsed.username or parsed.password:
        raise ValueError("R2 public URL must not contain credentials")
    prefix = f"utautts/{channel}/{deployment_id}"
    asset_base = public_url.rstrip("/") + "/" + prefix + "/"
    dependencies = asset_files(source)
    for name in (*HOST_FILES, "index.html", *dependencies):
        checked_file(source, name)
    if output.exists():
        shutil.rmtree(output)
    pages, r2 = output / "pages", output / "r2"
    app = pages / "app" / deployment_id
    app.mkdir(parents=True)
    r2.mkdir(parents=True)
    for name in HOST_FILES:
        shutil.copy2(source / name, app / name)
    (app / "config.js").write_text(
        "globalThis.UtauTTSConfig = " + json.dumps({"assetBaseURL": asset_base}) + ";\n", encoding="utf-8")
    html = (source / "index.html").read_text(encoding="utf-8")
    for name in ("config.js", "asset-paths.js", "bootstrap.js"):
        old = f'src="./{name}"'
        if old not in html:
            raise ValueError(f"index.html is missing {old}")
        html = html.replace(old, f'src="./app/{deployment_id}/{name}"')
    (pages / "index.html").write_text(html, encoding="utf-8")
    # 404ページを置き、欠落資産へのSPAフォールバックを防ぐ。
    (pages / "404.html").write_text("<!doctype html><meta charset=utf-8><title>404</title>Not found\n", encoding="utf-8")
    (pages / "_headers").write_text(
        "/*\n  X-Content-Type-Options: nosniff\n"
        "/\n  Cache-Control: no-cache\n/index.html\n  Cache-Control: no-cache\n"
        "/deployment.json\n  Cache-Control: no-cache\n"
        "/app/*\n  Cache-Control: public, max-age=31536000, immutable\n", encoding="utf-8")
    assets = []
    for name in dependencies:
        file = checked_file(source, name)
        destination = r2 / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(file, destination)
        assets.append({"path": name, "key": prefix + "/" + name, "size": file.stat().st_size,
                       "sha256": hashlib.sha256(file.read_bytes()).hexdigest(),
                       "content_type": content_type(file)})
    metadata = {"version": version, "revision": revision, "channel": channel,
                "deployment_id": deployment_id, "asset_base_url": asset_base,
                "r2_prefix": prefix, "assets": assets}
    write_json(output / "upload-manifest.json", metadata)
    write_json(pages / "deployment.json", {key: metadata[key] for key in metadata if key != "assets"})
    for file in pages.rglob("*"):
        if file.is_file() and file.stat().st_size > PAGE_LIMIT:
            raise ValueError(f"Pages asset exceeds 25 MiB: {file}")
    print(f"Pages: {sum(p.stat().st_size for p in pages.rglob('*') if p.is_file()):,} bytes")
    print(f"R2: {len(assets)} files, {sum(item['size'] for item in assets):,} bytes")
    print(f"Asset URL: {asset_base}")
    return metadata


def upload(output, bucket, account_id):
    # boto3用の認証情報は環境変数で渡す。
    import boto3
    from botocore.config import Config

    if not bucket or not account_id:
        raise ValueError("R2 bucket and Cloudflare account id are required")
    metadata = read_json(output / "upload-manifest.json")
    client = boto3.client("s3", endpoint_url=f"https://{account_id}.r2.cloudflarestorage.com",
                         region_name="auto", config=Config(
                             max_pool_connections=12, retries={"mode": "standard", "max_attempts": 5},
                             request_checksum_calculation="when_required",
                             response_checksum_validation="when_required"))

    def put(item):
        file = checked_file(output / "r2", item["path"])
        if file.stat().st_size != item["size"] or hashlib.sha256(file.read_bytes()).hexdigest() != item["sha256"]:
            raise ValueError(f"asset changed after packaging: {item['path']}")
        with file.open("rb") as body:
            client.put_object(Bucket=bucket, Key=item["key"], Body=body,
                              ContentType=item["content_type"], CacheControl=IMMUTABLE_CACHE,
                              Metadata={"sha256": item["sha256"]})
        head = client.head_object(Bucket=bucket, Key=item["key"])
        if head["ContentLength"] != item["size"] or head["Metadata"].get("sha256") != item["sha256"]:
            raise ValueError(f"R2 verification failed: {item['key']}")

    with ThreadPoolExecutor(max_workers=8) as pool:
        list(pool.map(put, metadata["assets"]))
    print(f"Uploaded and verified {len(metadata['assets'])} R2 objects in {metadata['r2_prefix']}")


def request_public(url, method="GET", headers=None):
    last_error = None
    for attempt in range(5):
        try:
            with urlopen(Request(url, method=method, headers=headers or {}), timeout=60) as response:
                return response.headers, response.read()
        except Exception as error:
            last_error = error
            time.sleep(2 ** attempt)
    raise RuntimeError(f"public asset check failed: {url}") from last_error


def check_assets(output, origin):
    metadata = read_json(output / "upload-manifest.json")
    samples = {"utautts.wasm", "web/dist/utautts.wasm", "web/dist/fs-shim.js",
               "web/dist/openjtalk/dict-manifest.json", "web/dist/models/manifest.json",
               "web/dist/voice/manifest.json", "renderer/utautts-world-phrase/renderer.json",
               "web/dist/openjtalk/utautts-openjtalk.wasm", "web/dist/world/utautts-world.wasm"}
    samples.add(next(item["path"] for item in metadata["assets"]
                     if item["path"].startswith("web/dist/voice/") and item["path"].endswith(".wav")))
    samples.add(next(item["path"] for item in metadata["assets"]
                     if item["path"].startswith("web/dist/openjtalk/dict/")))
    for item in metadata["assets"]:
        if item["path"] not in samples:
            continue
        url = metadata["asset_base_url"] + quote(item["path"], safe="/")
        headers, _ = request_public(url, "HEAD", {"Origin": origin})
        if headers.get("Access-Control-Allow-Origin") not in ("*", origin):
            raise ValueError(f"R2 CORS does not allow {origin}: {url}")
        if headers.get_content_type() != item["content_type"].split(";")[0]:
            raise ValueError(f"incorrect Content-Type for {url}: {headers.get('Content-Type')}")
        if int(headers.get("Content-Length", -1)) != item["size"]:
            raise ValueError(f"incorrect Content-Length for {url}")
    print(f"R2 public assets verified for {origin}")


def check_public(output, pages_url):
    metadata = read_json(output / "upload-manifest.json")
    parsed = urlsplit(pages_url)
    origin = f"{parsed.scheme}://{parsed.netloc}"
    base = pages_url.rstrip("/") + "/"
    _, data = request_public(base + "deployment.json")
    if json.loads(data)["deployment_id"] != metadata["deployment_id"]:
        raise ValueError("Pages is serving a different deployment")
    _, html = request_public(base)
    if f"app/{metadata['deployment_id']}/bootstrap.js".encode() not in html:
        raise ValueError("Pages entry point does not reference the packaged runtime")
    for name in HOST_FILES:
        request_public(base + f"app/{metadata['deployment_id']}/{name}", "HEAD")
    check_assets(output, origin)
    print(f"Pages entry point and R2 public assets verified: {base}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    plan = sub.add_parser("plan")
    plan.add_argument("--event", required=True)
    plan.add_argument("--ref", required=True)
    plan.add_argument("--preview-name", default="dev")
    plan.add_argument("--revision", required=True)
    plan.add_argument("--run-id", required=True)
    plan.add_argument("--attempt", required=True)
    plan.add_argument("--appinfo", type=Path, default=Path("internal/appinfo/appinfo.json"))
    package = sub.add_parser("package")
    package.add_argument("--source", type=Path, default=Path("build/web-release"))
    package.add_argument("--output", type=Path, default=Path("build/cloudflare"))
    package.add_argument("--public-url", required=True)
    package.add_argument("--deployment-id", required=True)
    package.add_argument("--version", required=True)
    package.add_argument("--revision", required=True)
    package.add_argument("--channel", choices=("releases", "previews"), required=True)
    put = sub.add_parser("upload")
    put.add_argument("--output", type=Path, default=Path("build/cloudflare"))
    put.add_argument("--bucket", default=os.environ.get("CF_R2_BUCKET"))
    put.add_argument("--account-id", default=os.environ.get("CLOUDFLARE_ACCOUNT_ID"))
    verify = sub.add_parser("verify")
    verify.add_argument("--output", type=Path, default=Path("build/cloudflare"))
    verify.add_argument("--pages-url", required=True)
    assets = sub.add_parser("verify-assets")
    assets.add_argument("--output", type=Path, default=Path("build/cloudflare"))
    assets.add_argument("--origin", required=True)
    args = parser.parse_args()
    if args.command == "plan":
        values = deployment_plan(args.event, args.ref, args.preview_name, args.revision,
                                 args.run_id, args.attempt, read_json(args.appinfo)["version"])
        for key, value in values.items():
            print(f"{key}={value}")
    elif args.command == "package":
        assemble(args.source, args.output, args.public_url, args.deployment_id,
                 args.version, args.revision, args.channel)
    elif args.command == "upload":
        upload(args.output, args.bucket, args.account_id)
    elif args.command == "verify":
        check_public(args.output, args.pages_url)
    else:
        check_assets(args.output, args.origin)


if __name__ == "__main__":
    main()
