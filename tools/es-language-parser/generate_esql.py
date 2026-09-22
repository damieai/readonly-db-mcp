#!/usr/bin/env python3
"""Reproduce complete, immutable ES|QL grammars, including upstream imports."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import urllib.request

sys.dont_write_bytecode = True
from generate import ROOT, HERE, TOOL_SHA, VERSIONS, sha, clean_go_label_markers

SOURCE = "x-pack/plugin/esql/src/main/antlr/"
OUTPUT = ROOT / "internal/dialects/elasticsearch/esqlparser/generated"
GRAMMARS = {
    "esql8/EsqlBaseLexer.g4": "472db4728772a36361f9213e40d97ad3d83b43ad8654c8db397326aee234d777",
    "esql8/EsqlBaseParser.g4": "4175438aede4aa86ff96a13581ecac827a3f3f5c3fb23af0f01941a42c3aa9fd",
    "esql9/EsqlBaseLexer.g4": "e93533d7ed0864d159aa6258305a1734f4272a7ee62d5bc8f7ebec361d8e7f93",
    "esql9/EsqlBaseParser.g4": "c9fad6c283a37bde3cdc56e3d53dc69265a59f3633900c6f9fec84f1e4b6f347",
    "esql9/lexer/ChangePoint.g4": "a5305794e81772f2ed2ec02f5a61acd19526699e4f4028bc581354143a7b1a38",
    "esql9/lexer/Enrich.g4": "d59dd63e6b9083839fe559f553dffebebf8bb82d267c2cf9e65f431616dfa2fa",
    "esql9/lexer/Explain.g4": "84487d391612c528194aa52d0ac212ac2053aa568366c306dfd980e6c3fc96e2",
    "esql9/lexer/Expression.g4": "021f7ef3f4a93bdf667d222426d0fae31c50ffc13de286ff054835352c913573",
    "esql9/lexer/Fork.g4": "49fa2770e51b260c23da5c0ca901dd936ef69d8554f164d67919c518d50188c6",
    "esql9/lexer/From.g4": "c9dbca68eaa74ddb8528318a1e9c60149a73e66d6e74363c4e47ca6b159da1a8",
    "esql9/lexer/Join.g4": "6eeb99bd278e3dc7f852a81f8f690ac4481e94832497722a0913ae7dc565234c",
    "esql9/lexer/Lookup.g4": "a7ab6888e2f55cdfae793d0fed53f1a3abd88b0e368013fdeeeed1f2919ed349",
    "esql9/lexer/MvExpand.g4": "63b74764c1c13443d8e60fa0d3ed95b2936c34ef00927a232473792318968a71",
    "esql9/lexer/Project.g4": "fc4967798c4e86d42cc13f2cc137f777decfcc164cdfda98c650dfd43f2b904b",
    "esql9/lexer/Rename.g4": "f698b314cccf8bc2ea603069985efdbff175f9d894436c5a5784876f9b070a32",
    "esql9/lexer/Rrf.g4": "ba7d04236da36a4d1f626d84a7b90741377394e310a405d607cbfa4783d4bfbb",
    "esql9/lexer/Show.g4": "233f27fb58e12c347944b16d27abed120d26e39689793674200a09282bab47aa",
    "esql9/lexer/UnknownCommand.g4": "ecdb8c9d202569644d2d343cd5463749cfb4c027fecded70812d812eecf08f77",
    "esql9/parser/Expression.g4": "f8e17a9541a780368fb8055141eb4a010464be43a59cba9f0ae71253c92f92af",
    "esql9/parser/Join.g4": "c421bbff177a9260ce19c44d65ec6eabf7dadc8aa3e5b0495c2323d75dcd49a8"
}
LICENSE_SHA = "48255018b41fc0e965b1115af7e6779bc218bb8a6747d561da800d5022622aa2"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--antlr-jar", type=Path)
    parser.add_argument("--java", default="java")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--verify-upstream", action="store_true")
    args = parser.parse_args()
    if args.antlr_jar and sha(args.antlr_jar.read_bytes()) != TOOL_SHA:
        raise SystemExit("ANTLR generator differs from pinned 4.13.1 artifact")
    if not args.antlr_jar and not args.check:
        parser.error("--antlr-jar is required to generate")
    if sha((HERE / "upstream/LICENSE-Elastic-2.0.txt").read_bytes()) != LICENSE_SHA:
        raise SystemExit("pinned ES|QL grammar license differs")
    profiles, outputs = {}, {}
    for version, commit in VERSIONS.items():
        major = version[0]
        prefix = "esql" + major + "/"
        grammars = {}
        for name, digest in GRAMMARS.items():
            if not name.startswith(prefix):
                continue
            path = HERE / "upstream" / name
            if sha(path.read_bytes()) != digest:
                raise SystemExit("pinned ES|QL grammar differs: " + name)
            url = f"https://raw.githubusercontent.com/elastic/elasticsearch/{commit}/{SOURCE}{name[len(prefix):]}"
            if args.verify_upstream:
                with urllib.request.urlopen(url, timeout=30) as response:
                    if sha(response.read(1 << 20)) != digest:
                        raise SystemExit("upstream ES|QL grammar differs: " + name)
            grammars[name[len(prefix):]] = {"source": url, "sha256": digest}
        profiles[version] = grammars
        if not args.antlr_jar:
            continue
        grammar_dir = HERE / "upstream" / ("esql" + major)
        with tempfile.TemporaryDirectory(prefix="readonly-esql-parser-") as temporary:
            directory = Path(temporary)
            # The lexer and parser import identically named grammars from
            # different directories; use separate ANTLR library directories.
            for kind in ("Lexer", "Parser"):
                library = directory / kind
                library.mkdir()
                for path in (grammar_dir / kind.lower()).glob("*.g4"):
                    shutil.copyfile(path, library / path.name)
                if kind == "Parser":
                    shutil.copyfile(directory / "EsqlBaseLexer.tokens", library / "EsqlBaseLexer.tokens")
                grammar = grammar_dir / ("EsqlBase" + kind + ".g4")
                subprocess.run([args.java, "-jar", str(args.antlr_jar.resolve()), "-Dlanguage=Go", "-package", "v" + major, "-listener", "-no-visitor", "-lib", str(library), "-Xexact-output-dir", "-o", str(directory), str(grammar.relative_to(ROOT))], cwd=ROOT, check=True)
            for path in sorted(directory.glob("*.go")):
                body = clean_go_label_markers(path.read_text(), "EsqlBaseParser")
                # Target-language spelling only. Preserve all dev predicates;
                # Config returns false just like the attested release builds.
                receiver = "p"
                body = body.replace("this.isDevVersion()", receiver + ".IsDevVersion()")
                path.write_text(body)
                subprocess.run(["gofmt", "-w", str(path)], check=True)
                outputs["v" + major + "/" + path.name] = path.read_bytes()
            config = directory / "config.go"
            config.write_text('// Code generated by tools/es-language-parser/generate_esql.py. DO NOT EDIT.\n\npackage v' + major + '\n\nimport "github.com/antlr4-go/antlr/v4"\n\n// Release profiles disable snapshot-only grammar predicates, as EsqlConfig does.\ntype LexerConfig struct { *antlr.BaseLexer }\nfunc (*LexerConfig) IsDevVersion() bool { return false }\ntype ParserConfig struct { *antlr.BaseParser }\nfunc (*ParserConfig) IsDevVersion() bool { return false }\n')
            subprocess.run(["gofmt", "-w", str(config)], check=True)
            outputs["v" + major + "/config.go"] = config.read_bytes()
    actual = {str(p.relative_to(OUTPUT)): p.read_bytes() for p in sorted(OUTPUT.glob("v*/*.go"))}
    manifest = {"profiles": profiles, "antlr_version": "4.13.1", "antlr_sha256": TOOL_SHA, "license_sha256": LICENSE_SHA, "generated_sha256": {name: sha(raw) for name, raw in sorted((outputs if args.antlr_jar else actual).items())}}
    encoded = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
    manifest_path = HERE / "esql-manifest.json"
    if args.check:
        if manifest_path.read_bytes() != encoded or (args.antlr_jar and outputs != actual):
            raise SystemExit("ES|QL parser differs from pinned manifest or reproduction")
    else:
        for name, raw in outputs.items():
            path = OUTPUT / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(raw)
        manifest_path.write_bytes(encoded)


if __name__ == "__main__":
    main()
