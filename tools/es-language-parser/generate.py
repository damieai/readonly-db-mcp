#!/usr/bin/env python3
"""Reproduce the pinned upstream EQL Go parser. No remote code runs implicitly."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
OUTPUT = ROOT / "internal/dialects/elasticsearch/eqlparser/generated"
GRAMMAR_SHA = "489354eb66e70ee7eafa4c54a8bd67a917e66bd16f129838d258aa8924c60267"
LICENSE_SHA = "48255018b41fc0e965b1115af7e6779bc218bb8a6747d561da800d5022622aa2"
TOOL_SHA = "bc13a9c57a8dd7d5196888211e5ede657cb64a3ce968608697e4f668251a8487"
VERSIONS = {"8.19.21": "4fe44c255c3d0da06779b921e132cdc555ed9aff", "9.1.10": "f2e019bd8110088070638ca779ec1543188c0f43"}
SOURCE = "x-pack/plugin/eql/src/main/antlr/EqlBase.g4"

def sha(data):
    return hashlib.sha256(data).hexdigest()

def clean_go_label_markers(source, parser_name="EqlBaseParser"):
    # ANTLR 4.13.1 emits a goto after each rule's return solely to keep an
    # otherwise unused errorExit label legal. Remove these unreachable markers;
    # also remove the label where no real branch uses it. Parsing transitions,
    # executable error handling and the upstream grammar remain unchanged.
    marker = "\tgoto errorExit // Trick to prevent compiler error if the label is not used\n"
    source = source.replace(marker, "")
    def rule(match):
        body = match.group(0)
        if "goto errorExit" not in body:
            body = body.replace("\nerrorExit:\n", "\n")
        return body
    return re.sub(r"(?m)^func \(p \*" + re.escape(parser_name) + r"\)[^\n]+\{\n.*?^\}", rule, source, flags=re.S)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--antlr-jar", type=Path)
    parser.add_argument("--java", default="java")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--verify-upstream", action="store_true")
    args = parser.parse_args()
    grammar = HERE / "upstream/EqlBase.g4"
    if sha(grammar.read_bytes()) != GRAMMAR_SHA or sha((HERE / "upstream/LICENSE-Elastic-2.0.txt").read_bytes()) != LICENSE_SHA:
        raise SystemExit("pinned upstream grammar/license differs")
    sources = {v: f"https://raw.githubusercontent.com/elastic/elasticsearch/{commit}/{SOURCE}" for v, commit in VERSIONS.items()}
    if args.verify_upstream:
        for url in sources.values():
            with urllib.request.urlopen(url, timeout=30) as response:
                if sha(response.read(1 << 20)) != GRAMMAR_SHA:
                    raise SystemExit("upstream grammar differs from reviewed profile")
    manifest_path = HERE / "manifest.json"
    if args.antlr_jar is None:
        if not args.check:
            parser.error("--antlr-jar is required to generate; --check also supports offline integrity checking")
        manifest = json.loads(manifest_path.read_text())
        if manifest["grammar_sha256"] != GRAMMAR_SHA or manifest["sources"] != sources or manifest["antlr_sha256"] != TOOL_SHA:
            raise SystemExit("parser source manifest differs")
        actual = {p.name: sha(p.read_bytes()) for p in sorted(OUTPUT.glob("*.go"))}
        if actual != manifest["generated_sha256"]:
            raise SystemExit("generated EQL parser differs from recorded hashes")
        return
    if sha(args.antlr_jar.read_bytes()) != TOOL_SHA:
        raise SystemExit("ANTLR generator differs from pinned 4.13.1 artifact")
    with tempfile.TemporaryDirectory(prefix="readonly-eql-parser-") as temporary:
        directory = Path(temporary)
        subprocess.run([args.java, "-jar", str(args.antlr_jar.resolve()), "-Dlanguage=Go", "-package", "generated", "-listener", "-no-visitor", "-Xexact-output-dir", "-o", str(directory), str(grammar.relative_to(ROOT))], cwd=ROOT, check=True)
        outputs = {}
        license_header = grammar.read_text().split("grammar EqlBase;", 1)[0].rstrip()
        for path in sorted(directory.glob("*.go")):
            first, body = clean_go_label_markers(path.read_text()).split("\n", 1)
            path.write_text(first + "\n\n" + license_header + "\n" + body)
            subprocess.run(["gofmt", "-w", str(path)], check=True)
            outputs[path.name] = path.read_bytes()
        manifest = {"grammar_sha256": GRAMMAR_SHA, "sources": sources, "antlr_version": "4.13.1", "antlr_sha256": TOOL_SHA, "generated_sha256": {name: sha(raw) for name, raw in outputs.items()}}
        encoded = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
        if args.check:
            if manifest_path.read_bytes() != encoded or set(outputs) != {p.name for p in OUTPUT.glob("*.go")}:
                raise SystemExit("parser manifest differs after regeneration")
            for name, raw in outputs.items():
                if (OUTPUT / name).read_bytes() != raw:
                    raise SystemExit("generated parser is not reproducible: " + name)
        else:
            OUTPUT.mkdir(parents=True, exist_ok=True)
            for name, raw in outputs.items():
                (OUTPUT / name).write_bytes(raw)
            manifest_path.write_bytes(encoded)

if __name__ == "__main__":
    main()
