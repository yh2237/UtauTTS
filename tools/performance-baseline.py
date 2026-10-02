"""代表ベンチとネイティブ合成の条件・結果を新しいディレクトリへ保存する。"""

import argparse
import hashlib
import json
import os
import platform
import shutil
import statistics
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PACKAGES = ["./internal/oto", "./internal/audio", "./internal/pitch",
            "./internal/acoustic", "./internal/render/base"]


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--voicebank", required=True)
    parser.add_argument("--corpus", default="tools/evaluation/japanese-v1.json")
    parser.add_argument("--model", default="models/frame-intonation-tcn-v10.json")
    parser.add_argument("--count", type=int, default=5)
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--benchtime", default="200ms")
    parser.add_argument("--gomaxprocs", type=int, default=4)
    args = parser.parse_args()
    if min(args.count, args.repeat, args.gomaxprocs) < 1:
        parser.error("count, repeat and gomaxprocs must be positive")
    out = (ROOT / args.out).resolve()
    bank = (ROOT / args.voicebank).resolve()
    corpus = (ROOT / args.corpus).resolve()
    model = (ROOT / args.model).resolve()
    for path in (bank, corpus, model):
        if not path.exists():
            parser.error(f"missing input: {path}")
    out.mkdir(parents=True, exist_ok=False)
    env = dict(os.environ, GOMAXPROCS=str(args.gomaxprocs))
    commands = []

    def run(command, log, command_env=None):
        print("RUN", subprocess.list2cmdline([str(x) for x in command]), flush=True)
        result = subprocess.run([str(x) for x in command], cwd=ROOT, env=command_env or env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        (out / log).write_bytes(result.stdout)
        if result.stderr:
            (out / (log + ".stderr")).write_bytes(result.stderr)
        commands.append({"command": [str(x) for x in command], "log": log,
                         "exit_code": result.returncode})
        (out / "commands.json").write_text(json.dumps(commands, indent=2), encoding="utf-8")
        if result.returncode:
            print((result.stdout + result.stderr).decode("utf-8", errors="replace"), file=sys.stderr)
            raise RuntimeError(f"command failed; see {out / log}")
        return result.stdout.decode("utf-8")

    suffix = ".exe" if os.name == "nt" else ""
    binary = out / ("tts-eval" + suffix)
    bridge = out / ("worldline-bridge" + suffix)
    metadata = {"scope": "native Japanese baseline, not GUI startup or browser",
                "platform": platform.platform(), "processor": platform.processor(),
                "logical_cpus": os.cpu_count(), "gomaxprocs": args.gomaxprocs,
                "benchmark_cpu": 1, "arguments": vars(args), "voicebank": str(bank),
                "corpus_sha256": sha256(corpus), "model_sha256": sha256(model)}
    metadata["commit"] = run(["git", "rev-parse", "HEAD"], "commit.txt").strip()
    metadata["worktree"] = run(["git", "status", "--porcelain"], "worktree.txt")
    metadata["go_version"] = run(["go", "version"], "go-version.txt").strip()
    run(["git", "diff", "--binary"], "source.patch")
    modified = run(["git", "diff", "--name-only", "--diff-filter=AM"], "modified-files.txt")
    untracked = run(["git", "ls-files", "--others", "--exclude-standard"], "new-files.txt")
    for name in set((modified + untracked).splitlines()):
        destination = out / "_source" / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, destination)
    (out / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8")
    run(["go", "env", "-json", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS"], "go-env.json")
    run(["go", "list", "-m", "-json", "all"], "modules.jsonl")
    run(["go", "list", "-deps", "-json", "./cmd/utautts-cli"], "cli-dependencies.jsonl")
    assets = []
    for path in sorted(bank.rglob("*")):
        if path.is_file():
            assets.append({"path": path.relative_to(bank).as_posix(),
                           "bytes": path.stat().st_size, "sha256": sha256(path)})
    for path in sorted((ROOT / "runtime").glob("utautts-world-engine.*")):
        if path.is_file():
            assets.append({"path": str(path), "bytes": path.stat().st_size, "sha256": sha256(path)})
    helper = ROOT / "tools/openjtalk-feature-bridge/bin" / ("utautts-openjtalk-features" + suffix)
    if helper.is_file():
        assets.append({"path": str(helper), "bytes": helper.stat().st_size, "sha256": sha256(helper)})
    (out / "input-assets.json").write_text(json.dumps(assets, ensure_ascii=False, indent=2), encoding="utf-8")
    run(["go", "test", "-run", "^$", "-bench", ".", "-benchmem", "-cpu", "1",
         "-count", str(args.count), "-benchtime", args.benchtime, *PACKAGES], "bench.txt")
    run(["go", "build", "-o", binary, "./cmd/tools/tts-eval"], "build-eval.txt")
    run(["go", "build", "-o", bridge, "./cmd/utautts-worldline-bridge"], "build-bridge.txt")
    cli = out / ("utautts-cli" + suffix)
    wasm = out / "utautts.wasm"
    run(["go", "build", "-o", cli, "./cmd/utautts-cli"], "build-cli.txt")
    run(["go", "build", "-o", wasm, "./cmd/utautts-wasm"], "build-wasm.txt",
        dict(env, GOOS="js", GOARCH="wasm", CGO_ENABLED="0"))
    metadata["binaries"] = [{"path": path.name, "bytes": path.stat().st_size,
                             "sha256": sha256(path)} for path in (binary, bridge, cli, wasm)]
    (out / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8")
    command = [binary, "--voicebank", bank, "--corpus", corpus, "--model-file", model,
               "--bridge", bridge, "--repeat", str(args.repeat)]
    run([*command, "--out", out / "timing"], "timing.txt")
    run([*command, "--out", out / "profile", "--profile"], "profile.txt")
    run(["go", "tool", "pprof", "-top", binary, out / "profile/cpu.pprof"], "cpu-top.txt")
    run(["go", "tool", "pprof", "-top", "-alloc_space", binary,
         out / "profile/allocs.pprof"], "allocs-top.txt")
    rows = json.loads((out / "timing/report.json").read_text(encoding="utf-8"))["Measurements"]
    summary = []
    for identifier in dict.fromkeys(row["id"] for row in rows):
        cases = [row for row in rows if row["id"] == identifier]
        warm = [row["elapsed_ms"] for row in cases if row["repetition"] > 1]
        summary.append({"id": identifier, "first_ms": cases[0]["elapsed_ms"],
                        "warm_median_ms": statistics.median(warm) if warm else None,
                        "audio_ms": cases[0]["audio_ms"]})
    (out / "summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    (out / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    print("Saved", out)


if __name__ == "__main__":
    main()
