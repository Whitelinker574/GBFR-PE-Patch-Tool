#!/usr/bin/env python3
"""One-off Qiniu/QNAIGC GPT Image 2 image-edit helper.

The API key is read from QNAIGC_API_KEY or entered with a hidden prompt. It is
never stored by this script. The generated PNG and a small, secret-free metadata
file are written to disk.
"""

from __future__ import annotations

import argparse
import base64
import getpass
import hashlib
import http.client
import json
import mimetypes
import os
from pathlib import Path
import subprocess
import sys
import time
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


API_URL = "https://api.qnaigc.com/v1/images/edits"
DEFAULT_MODEL = "openai/gpt-image-2"
TRANSIENT_CODES = {408, 409, 425, 429, 500, 502, 503, 504}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Call QNAIGC GPT Image 2 image-to-image without storing the API key."
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--manifest", type=Path, help="Batch JSON manifest.")
    source.add_argument("--image", action="append", type=Path, help="Reference image; repeatable.")
    parser.add_argument("--prompt", help="Edit prompt for single-image mode.")
    parser.add_argument("--prompt-file", type=Path, help="UTF-8 prompt file for single-image mode.")
    parser.add_argument("--output", type=Path, help="Output PNG for single-image mode.")
    parser.add_argument("--size", default="3840x2160", help="Output size, e.g. 3840x2160.")
    parser.add_argument("--quality", choices=("low", "medium", "high", "auto"), default="high")
    parser.add_argument("--background", choices=("transparent", "opaque", "auto"), default="auto")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--only", action="append", help="In manifest mode, run only named jobs.")
    parser.add_argument("--timeout", type=int, default=900)
    parser.add_argument("--retries", type=int, default=3)
    return parser.parse_args()


def get_api_key() -> str:
    key = os.environ.get("QNAIGC_API_KEY", "").strip()
    if not key:
        if not sys.stdin.isatty():
            raise RuntimeError(
                "QNAIGC_API_KEY is not set and no interactive terminal is available."
            )
        key = getpass.getpass("QNAIGC API key (hidden, not saved): ").strip()
    if not key:
        raise RuntimeError("No API key supplied.")
    return key


def as_data_uri(path: Path) -> str:
    if not path.is_file():
        raise FileNotFoundError(path)
    mime = mimetypes.guess_type(path.name)[0] or "image/png"
    encoded = base64.b64encode(path.read_bytes()).decode("ascii")
    return f"data:{mime};base64,{encoded}"


def safe_error_body(raw: bytes) -> str:
    text = raw.decode("utf-8", errors="replace")[:1200]
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return text
    for field in ("key", "api_key", "authorization", "token"):
        if field in payload:
            payload[field] = "[redacted]"
    return json.dumps(payload, ensure_ascii=False)[:1200]


def post_edit(payload: dict[str, Any], api_key: str, timeout: int, retries: int) -> dict[str, Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    for attempt in range(retries + 1):
        request = Request(
            API_URL,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
                "Accept": "application/json",
                "User-Agent": "GBFR-PE-Patch-Tool-local-art-helper/1.0",
            },
        )
        try:
            with urlopen(request, timeout=timeout) as response:
                return json.loads(response.read().decode("utf-8"))
        except HTTPError as exc:
            detail = safe_error_body(exc.read())
            if exc.code not in TRANSIENT_CODES or attempt == retries:
                raise RuntimeError(f"QNAIGC HTTP {exc.code}: {detail}") from exc
        except (URLError, TimeoutError, ConnectionError, http.client.RemoteDisconnected) as exc:
            if attempt == retries:
                detail = exc.reason if isinstance(exc, URLError) else exc
                raise RuntimeError(f"QNAIGC request failed: {detail}") from exc
        delay = min(2 ** (attempt + 1), 20)
        print(f"Transient API error; retrying in {delay}s ({attempt + 1}/{retries})...", flush=True)
        time.sleep(delay)
    raise AssertionError("retry loop exhausted")


def resolve_path(base: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else (base / path).resolve()


def save_result(result: dict[str, Any], output: Path, job: dict[str, Any]) -> None:
    data = result.get("data") or []
    if not data or not isinstance(data[0], dict):
        raise RuntimeError("QNAIGC response did not contain image data.")
    item = data[0]
    output.parent.mkdir(parents=True, exist_ok=True)
    if item.get("b64_json"):
        output.write_bytes(base64.b64decode(item["b64_json"]))
    elif item.get("url"):
        with urlopen(item["url"], timeout=300) as response:
            output.write_bytes(response.read())
    else:
        raise RuntimeError("QNAIGC response contained neither b64_json nor url.")

    metadata = {
        "model": job["model"],
        "size": result.get("size", job["size"]),
        "quality": result.get("quality", job["quality"]),
        "background": result.get("background", job["background"]),
        "output_format": result.get("output_format", "png"),
        "prompt_sha256": hashlib.sha256(job["prompt"].encode("utf-8")).hexdigest(),
        "source_files": [Path(path).name for path in job["images"]],
        "usage": result.get("usage", {}),
    }
    output.with_suffix(output.suffix + ".json").write_text(
        json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(f"Saved {output} ({output.stat().st_size:,} bytes)", flush=True)


def run_job(job: dict[str, Any], api_key: str, timeout: int, retries: int) -> None:
    print(f"Generating {job['name']} at {job['size']} / {job['quality']}...", flush=True)
    prompt = job["prompt"]
    if job.get("prompt_suffix"):
        prompt += " " + str(job["prompt_suffix"]).strip()
    if job.get("chroma_key"):
        prompt += (
            " API does not support alpha for this model: ignore any earlier transparent-background "
            "instruction and instead render the subject on one perfectly flat, evenly lit, solid "
            "chroma green #00FF00 background. No gradient, shadow, texture, horizon, floor, green rim "
            "light, green reflections, or green objects near the silhouette."
        )
    payload = {
        "model": job["model"],
        "prompt": prompt,
        "image": [as_data_uri(Path(path)) for path in job["images"]],
        "size": job["size"],
        "quality": job["quality"],
        "background": job["background"],
        "output_format": "png",
    }
    result = post_edit(payload, api_key, timeout, retries)
    output = Path(job["output"])
    if not job.get("chroma_key"):
        save_result(result, output, job)
        return

    raw_output = output.with_name(f"{output.stem}-key{output.suffix}")
    save_result(result, raw_output, job)
    helper = Path.home() / ".codex/skills/.system/imagegen/scripts/remove_chroma_key.py"
    if not helper.is_file():
        raise RuntimeError(f"Chroma-key helper not found: {helper}")
    subprocess.run(
        [
            sys.executable,
            str(helper),
            "--input", str(raw_output),
            "--out", str(output),
            "--auto-key", "border",
            "--soft-matte",
            "--transparent-threshold", "18",
            "--opaque-threshold", "104",
            "--edge-contract", "1",
            "--edge-feather", "0.45",
            "--despill",
            "--force",
        ],
        check=True,
    )
    raw_metadata = raw_output.with_suffix(raw_output.suffix + ".json")
    final_metadata = output.with_suffix(output.suffix + ".json")
    if final_metadata.exists():
        final_metadata.unlink()
    raw_metadata.replace(final_metadata)
    raw_output.unlink()


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    parent_name = manifest.get("extends")
    if not parent_name:
        return manifest
    parent_path = resolve_path(path.parent, parent_name)
    parent = load_manifest(parent_path)
    merged = {
        **parent,
        **manifest,
        "defaults": {**parent.get("defaults", {}), **manifest.get("defaults", {})},
        "jobs": manifest.get("jobs", parent.get("jobs", [])),
    }
    overrides = manifest.get("job_overrides", {})
    if overrides:
        merged["jobs"] = [
            {**job, **overrides.get(job.get("name"), {})}
            for job in merged.get("jobs", [])
        ]
    return merged


def jobs_from_manifest(path: Path, args: argparse.Namespace) -> list[dict[str, Any]]:
    manifest_path = path.resolve()
    base = manifest_path.parent
    manifest = load_manifest(manifest_path)
    defaults = manifest.get("defaults", {})
    output_dir = manifest.get("output_dir")
    selected = set(args.only or [])
    jobs: list[dict[str, Any]] = []
    for raw in manifest.get("jobs", []):
        if selected and raw.get("name") not in selected:
            continue
        job = {**defaults, **raw}
        job["images"] = [str(resolve_path(base, item)) for item in job["images"]]
        if output_dir:
            job["output"] = str(resolve_path(base, Path(output_dir) / Path(job["output"]).name))
        else:
            job["output"] = str(resolve_path(base, job["output"]))
        job.setdefault("model", args.model)
        job.setdefault("size", args.size)
        job.setdefault("quality", args.quality)
        job.setdefault("background", args.background)
        jobs.append(job)
    if not jobs:
        raise RuntimeError("Manifest selected no jobs.")
    return jobs


def single_job(args: argparse.Namespace) -> dict[str, Any]:
    if not args.output:
        raise RuntimeError("--output is required in single-image mode.")
    if bool(args.prompt) == bool(args.prompt_file):
        raise RuntimeError("Provide exactly one of --prompt or --prompt-file.")
    prompt = args.prompt or args.prompt_file.read_text(encoding="utf-8")
    return {
        "name": args.output.stem,
        "images": [str(path.resolve()) for path in args.image],
        "prompt": prompt,
        "output": str(args.output.resolve()),
        "model": args.model,
        "size": args.size,
        "quality": args.quality,
        "background": args.background,
    }


def main() -> int:
    args = parse_args()
    try:
        jobs = jobs_from_manifest(args.manifest, args) if args.manifest else [single_job(args)]
        api_key = get_api_key()
        for job in jobs:
            run_job(job, api_key, args.timeout, args.retries)
    except (OSError, ValueError, RuntimeError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
