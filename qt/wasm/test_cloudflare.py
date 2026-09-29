import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from email.message import Message


spec = importlib.util.spec_from_file_location("cloudflare", Path(__file__).with_name("cloudflare.py"))
cloudflare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cloudflare)


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "release"
        self.source.mkdir()
        self.output = self.root / "cloudflare"
        for name in cloudflare.HOST_FILES:
            self.write(name, "// host")
        self.write("index.html", '<script src="./config.js"></script><script src="./asset-paths.js"></script><script src="./bootstrap.js"></script>')
        self.write("web/dist/models/manifest.json", json.dumps({"models": ["model.json"]}))
        self.write("web/dist/openjtalk/dict-manifest.json", json.dumps({"files": ["sys.dic"]}))
        self.write("web/dist/voice/manifest.json", json.dumps({"files": {"足立レイ/a.wav": 4}}))
        self.write("web/dist/models/model.json", "{}")
        self.write("web/dist/openjtalk/dict/sys.dic", "dict")
        self.write("web/dist/voice/足立レイ/a.wav", "RIFF")
        for name in cloudflare.asset_files(self.source):
            if not (self.source / name).exists():
                self.write(name, "asset")

    def write(self, name, value):
        file = self.source / name
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_text(value, encoding="utf-8")

    def assemble(self, **overrides):
        args = dict(source=self.source, output=self.output, public_url="https://assets.example.test",
                    deployment_id="v1.2.3-abc-123-1", version="v1.2.3", revision="a" * 40, channel="releases")
        args.update(overrides)
        return cloudflare.assemble(**args)

    def test_split_and_version_pinning_exclude_debug_output(self):
        self.write("web/dist/out/private.wav", "do not publish")
        metadata = self.assemble()
        app = self.output / "pages/app/v1.2.3-abc-123-1"
        self.assertIn(metadata["asset_base_url"], (app / "config.js").read_text())
        self.assertIn('./app/v1.2.3-abc-123-1/bootstrap.js', (self.output / "pages/index.html").read_text())
        self.assertFalse((self.output / "pages/utautts.wasm").exists())
        self.assertTrue((self.output / "r2/utautts.wasm").exists())
        self.assertFalse((self.output / "r2/web/dist/out").exists())
        self.assertTrue((self.output / "pages/404.html").exists())
        wasm = next(item for item in metadata["assets"] if item["path"] == "utautts.wasm")
        self.assertEqual(wasm["content_type"], "application/wasm")
        self.assertTrue(wasm["key"].startswith("utautts/releases/v1.2.3-abc-123-1/"))

    def test_large_wasm_goes_to_r2_but_large_pages_script_is_rejected(self):
        with (self.source / "utautts.wasm").open("wb") as file:
            file.seek(cloudflare.PAGE_LIMIT)
            file.write(b"x")
        self.assemble()
        with (self.source / "utautts.js").open("wb") as file:
            file.seek(cloudflare.PAGE_LIMIT)
            file.write(b"x")
        with self.assertRaisesRegex(ValueError, "25 MiB"):
            self.assemble()

    def test_missing_dependency_or_manifest_traversal_is_rejected(self):
        (self.source / "web/dist/world/utautts-world.wasm").unlink()
        with self.assertRaisesRegex(ValueError, "missing or invalid"):
            self.assemble()
        self.write("web/dist/world/utautts-world.wasm", "asset")
        self.write("web/dist/models/manifest.json", json.dumps({"models": ["../../../../outside.json"]}))
        with self.assertRaisesRegex(ValueError, "missing or invalid"):
            self.assemble()

    def test_invalid_output_id_and_url_are_rejected(self):
        for overrides in ({"output": self.source}, {"deployment_id": "../unsafe"},
                          {"public_url": "https://secret:token@example.test/"},
                          {"public_url": "relative/assets"}):
            with self.subTest(overrides=overrides), self.assertRaises(ValueError):
                self.assemble(**overrides)

    def test_tag_production_and_manual_preview(self):
        plan = cloudflare.deployment_plan("push", "refs/tags/v1.2.3", "dev", "a" * 40, "123", "1", "v1.2.3")
        self.assertEqual(plan["branch"], "main")
        self.assertEqual(plan["channel"], "releases")
        plan = cloudflare.deployment_plan("workflow_dispatch", "refs/heads/main", "touch", "a" * 40, "123", "1", "v1.2.3")
        self.assertEqual(plan["branch"], "preview-touch")
        self.assertEqual(plan["channel"], "previews")

    def test_only_three_part_numeric_versions_are_accepted(self):
        invalid_versions = ("v1.2.3-rc.1", "v1.2.3-beta", "v1.2.3+build.1",
                            "v1.2", "v1.2.3.4", "1.2.3", "v１.2.3")
        for version in invalid_versions:
            with self.subTest(tag=version), self.assertRaisesRegex(ValueError, "release tag"):
                cloudflare.deployment_plan("push", "refs/tags/" + version, "dev", "a" * 40, "123", "1", "v1.2.3")
            with self.subTest(appinfo=version), self.assertRaisesRegex(ValueError, "appinfo version"):
                cloudflare.deployment_plan("workflow_dispatch", "refs/heads/main", "dev", "a" * 40, "123", "1", version)

    def test_normal_push_cannot_deploy_and_version_mismatch_fails(self):
        for event, ref, preview, version in (
            ("push", "refs/heads/main", "dev", "v1.2.3"),
            ("push", "refs/tags/v1.2.4", "dev", "v1.2.3"),
            ("workflow_dispatch", "refs/heads/main", "main;command", "v1.2.3"),
        ):
            with self.subTest(event=event, ref=ref), self.assertRaises(ValueError):
                cloudflare.deployment_plan(event, ref, preview, "a" * 40, "123", "1", version)

    def test_public_asset_validation_checks_cors_and_mime(self):
        metadata = self.assemble()
        by_url = {metadata["asset_base_url"] + item["path"]: item for item in metadata["assets"]}

        def response(url, method, headers):
            from urllib.parse import unquote
            item = by_url[unquote(url)]
            result = Message()
            result["Access-Control-Allow-Origin"] = "*"
            result["Content-Type"] = item["content_type"]
            result["Content-Length"] = str(item["size"])
            return result, b""

        with patch.object(cloudflare, "request_public", side_effect=response):
            cloudflare.check_assets(self.output, "https://tts.pages.dev")
        def bad_cors(*args):
            headers, body = response(*args)
            del headers["Access-Control-Allow-Origin"]
            return headers, body
        with patch.object(cloudflare, "request_public", side_effect=bad_cors):
            with self.assertRaisesRegex(ValueError, "CORS"):
                cloudflare.check_assets(self.output, "https://tts.pages.dev")


if __name__ == "__main__":
    unittest.main()
