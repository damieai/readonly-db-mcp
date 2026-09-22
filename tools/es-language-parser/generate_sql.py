#!/usr/bin/env python3
"""Reproduce each pinned Elasticsearch SQL grammar without dialect substitution."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.request

sys.dont_write_bytecode = True
from generate import ROOT, HERE, TOOL_SHA, VERSIONS, sha, clean_go_label_markers

SOURCE = "x-pack/plugin/sql/src/main/antlr/SqlBase.g4"
OUTPUT = ROOT / "internal/dialects/elasticsearch/sqlparser/generated"
GRAMMARS = {
    "8.19.21": "89697dd145de475a56c9ca5d0a9c9990a2c69bb769a21e19fc3f656061e02d6f",
    "9.1.10": "a54ef906a572cec47690fcd7f23bfdf35373619a5ad3d969cb42cc04c69593c7",
}
LICENSE_SHA = "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--antlr-jar", type=Path)
    parser.add_argument("--java", default="java")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--verify-upstream", action="store_true")
    args = parser.parse_args()
    if sha((HERE / "upstream/LICENSE-Apache-2.0.txt").read_bytes()) != LICENSE_SHA:
        raise SystemExit("pinned SQL grammar license differs")
    if args.antlr_jar and sha(args.antlr_jar.read_bytes()) != TOOL_SHA:
        raise SystemExit("ANTLR generator differs from pinned 4.13.1 artifact")
    if not args.antlr_jar and not args.check:
        parser.error("--antlr-jar is required to generate")
    profiles = {}
    outputs = {}
    for version, commit in VERSIONS.items():
        major = version[0]
        grammar = HERE / ("upstream/sql" + major) / "SqlBase.g4"
        if sha(grammar.read_bytes()) != GRAMMARS[version]:
            raise SystemExit("pinned SQL grammar differs: " + version)
        url = f"https://raw.githubusercontent.com/elastic/elasticsearch/{commit}/{SOURCE}"
        if args.verify_upstream:
            with urllib.request.urlopen(url, timeout=30) as response:
                if sha(response.read(1 << 20)) != GRAMMARS[version]:
                    raise SystemExit("upstream SQL grammar differs: " + version)
        profiles[version] = {"source": url, "grammar_sha256": GRAMMARS[version]}
        if not args.antlr_jar:
            continue
        with tempfile.TemporaryDirectory(prefix="readonly-sql-parser-") as temporary:
            directory = Path(temporary)
            subprocess.run([args.java, "-jar", str(args.antlr_jar.resolve()), "-Dlanguage=Go", "-package", "v" + major, "-listener", "-no-visitor", "-Xexact-output-dir", "-o", str(directory), str(grammar.relative_to(ROOT))], cwd=ROOT, check=True)
            header = grammar.read_text().split("grammar SqlBase;", 1)[0].rstrip()
            for path in sorted(directory.glob("*.go")):
                first, body = clean_go_label_markers(path.read_text(), "SqlBaseParser").split("\n", 1)
                path.write_text(first + "\n\n" + header + "\n" + body)
                subprocess.run(["gofmt", "-w", str(path)], check=True)
                outputs["v" + major + "/" + path.name] = path.read_bytes()
    actual = {str(p.relative_to(OUTPUT)): p.read_bytes() for p in sorted(OUTPUT.glob("v*/*.go"))}
    manifest = {"profiles": profiles, "antlr_version": "4.13.1", "antlr_sha256": TOOL_SHA, "license_sha256": LICENSE_SHA, "generated_sha256": {name: sha(raw) for name, raw in sorted((outputs if args.antlr_jar else actual).items())}}
    encoded = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
    manifest_path = HERE / "sql-manifest.json"
    if args.check:
        if manifest_path.read_bytes() != encoded or (args.antlr_jar and outputs != actual):
            raise SystemExit("SQL parser differs from pinned manifest or reproduction")
    else:
        for name, raw in outputs.items():
            path = OUTPUT / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(raw)
        manifest_path.write_bytes(encoded)


if __name__ == "__main__":
    main()
