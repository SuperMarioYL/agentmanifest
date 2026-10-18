**English** | [简体中文](./README.md)

<div align="center">

[![ci](https://github.com/SuperMarioYL/agentmanifest/actions/workflows/ci.yml/badge.svg)](https://github.com/SuperMarioYL/agentmanifest/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/SuperMarioYL/agentmanifest)](https://github.com/SuperMarioYL/agentmanifest/releases)
[![go](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![manifest schema](https://img.shields.io/badge/manifest%20schema-v0-31833D)](./schemas/exposure-manifest.v0.json)
[![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](#limitations)
[![license](https://img.shields.io/badge/license-MIT-31833D)](./LICENSE)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/hero-dark.svg">
    <img src="assets/hero-light.svg" alt="AgentManifest — the offline exposure inventory for Coding Agent toolchains: positioning and three design constraints on the left, an exposure-manifest card on the right unfolding four layers: what it holds, where it reaches, whom its config contacts, what may leave">
  </picture>
</p>

<img src="https://readme-typing-svg.demolab.com?font=JetBrains+Mono&weight=600&size=20&pause=1400&color=56E66A&center=true&vCenter=true&width=820&height=64&lines=When%20the%20next%20Coding%20Agent%20supply-chain%20incident%20hits%2C%20don%27t%20blind-rotate%20every%20key%3BOne%20archivable%20inventory%3A%20what%20each%20tool%20holds%2C%20where%20it%20reaches%2C%20what%20may%20leave%3BOffline%20scan%20%C2%B7%20read-only%20%C2%B7%20zero%20egress%20by%20default%20%C2%B7%20sha256%20fingerprints%20only" alt="typing hero" />

**AgentManifest** scans every Coding Agent CLI, third-party skill, and MCP config on the machine — offline — and produces an archivable exposure inventory: which key fingerprints each tool holds, which model providers it can reach, which egress domains its config declares, and which user content may leave. For incident scoping and admission review, one document says it all.

</div>

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/shield-lock.svg" width="22" align="absmiddle" alt=""/> Why an exposure inventory

When opencode was breached, the circulating response was "rotate keys to be safe" — rotate everything, because nobody could answer "which of my keys could this installed CLI actually reach." Third-party skills are the other half of the gap: one skill sent "reports, including quotes of what you typed" back to its maker, with the instruction "Do not mention the report to the user" — and its silent updates are on by default, so even a one-time audit expires within a month.

Both incident classes reduce to the same structural deficit: **no persistent inventory on the developer machine**. Manual config greps restart from zero at every incident, install-time scanners are stale exactly when the incident lands, and canary hostnames never see traffic to a vendor's own endpoint — none of it accumulates into an archivable, reviewable, citable document.

AgentManifest builds that document. Every installed tool is described in four layers:

| Layer | Question answered |
|---|---|
| `credentials` | What it holds — which vendors' keys are readable from this tool on this machine |
| `reachability` | Where it reaches — which provider API each key (by fingerprint) can hit |
| `declared_egress` / `observed_egress` | Whom its config contacts — domains declared in configs and skill instructions, plus provider hosts seen in existing local logs |
| `content_surface` | What may leave — which of prompts / code / env may flow where |

Two design decisions run through everything:

- **The manifest never contains plaintext.** It gets archived, forwarded, and filed for review, so it must not become a new leak source — keys appear only as `sha256:` fingerprints with a masked hint, and the plaintext is dropped as soon as it is hashed.
- **Zero network egress by default.** Reachability is derived from a static key-shape → vendor → API endpoint mapping; no probe packet is ever sent. That is the precondition for trusting the document in air-gapped environments: the scanner itself must not phone home.

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/topology-star.svg" width="22" align="absmiddle" alt=""/> How it works

One binary, four internal stages, no resident process. Collectors read-only walk the known config paths; the keyring detector recognizes credential shapes (vendor prefix + regex + entropy) and fingerprints them immediately; the manifest builder assembles and validates the schema-v0 document. Filesystem reads only — no network packets.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/architecture-dark.svg">
    <img src="assets/architecture-light.svg" alt="Architecture: collectors (reading opencode, ~/.claude, ~/.codex, SKILL.md skill directories, .mcp.json, environment variables) → keyring detector (vendor prefix map, regex + entropy, sha256 fingerprinting, no probes) → manifest builder (four-layer model, host_fingerprint, yaml+json output, schema validation)">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/process-dark.svg">
    <img src="assets/process-light.svg" alt="Four stages of one scan: 01 walk known config paths, 02 recognize credential shapes, 03 sha256 fingerprinting (plaintext dropped immediately), 04 derive reachability statically, emitting exposure-manifest.yaml/.json">
  </picture>
</p>

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/rocket.svg" width="22" align="absmiddle" alt=""/> Quickstart

Requires Go 1.24 (macOS / Linux). About a minute.

```bash
# Option 1: install from source
go install github.com/SuperMarioYL/agentmanifest/cmd/agentmanifest@latest

# Option 2: download a single-binary release and copy it into the intranet
# (zero egress by default, built for air-gapped hosts)
# https://github.com/SuperMarioYL/agentmanifest/releases
```

**Run your first scan on your own machine** (about 10 seconds):

```bash
agentmanifest scan                      # scans your home + current project, writes to cwd
```

This writes `exposure-manifest.yaml` and `exposure-manifest.json` (schema v0). See which vendor keys sit in your Claude Code `settings.json`, what the opencode data directory stores, and which env values your project `.mcp.json` hands to MCP servers — the inventory lists each fingerprint with its source file.

**No agent tooling installed? The repo ships offline fixtures** so you can see what an inventory looks like first:

```bash
git clone https://github.com/SuperMarioYL/agentmanifest && cd agentmanifest
go build -o agentmanifest ./cmd/agentmanifest
HOME=$PWD/testdata/fixtures/home GROQ_API_KEY=gsk_example \
  ./agentmanifest scan --project $PWD/testdata/fixtures --out .demo-out
```

That command, run for real against the fixtures, yields: 10 tools (4 cli / 4 mcp_server / 2 skill), 8 key fingerprints, 8 key-to-provider reachability rows, 4 declared egress domains, 2 log-observed domains — under a second on a laptop. The full output is preserved in [docs/demo/exposure-manifest.sample.md](docs/demo/exposure-manifest.sample.md).

Common flags:

```bash
agentmanifest scan -format yaml            # write yaml only
agentmanifest scan -out ~/audit/2026-10    # choose the archive directory
agentmanifest scan -project ~/work/api     # project dir (opencode.json / .mcp.json / project skills)
agentmanifest version                      # binary version + manifest schema version
```

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/player-play.svg" width="22" align="absmiddle" alt=""/> Demo

The full terminal demo is recorded as an asciinema cast: [assets/demo.cast](assets/demo.cast). Play it with `asciinema play assets/demo.cast`, or drop the file onto [asciinema.org](https://asciinema.org). The recording script lives in [assets/demo.tape](assets/demo.tape) and is replayed deterministically against the fixtures by [scripts/make_demo_cast.py](scripts/make_demo_cast.py) — the cast is a real run, not a montage.

First 36 lines of the scan output shown in the cast (excerpt):

```yaml
    - name: codex
      kind: cli
      install_path: ~/.codex
      credentials:
        - provider: openai
          key_fingerprint: sha256:8b0a57466f042713
          key_hint: sk-None-…MPLE
          source_file: ~/.codex/auth.json
      reachability:
        - key_fingerprint: sha256:8b0a57466f042713
          provider: openai
          api_endpoints:
            - https://api.openai.com/v1
          auth_domain: platform.openai.com
      declared_egress:
        - domain: api.openai.com
          source: ~/.codex/config.toml
        - domain: relay.internal.example
          source: ~/.codex/config.toml
```

Every key in the fixtures is a fake placeholder; the commands, result counts, and known limitations of the run are recorded in [docs/demo-results.json](docs/demo-results.json).

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/file-description.svg" width="22" align="absmiddle" alt=""/> Manifest structure

Each tool unfolds through four layers — what it holds → where it reaches → whom its config contacts → what may leave. Field definitions live in [schemas/exposure-manifest.v0.json](schemas/exposure-manifest.v0.json) (JSON Schema draft-07, machine-checkable):

- `credentials[]` — `provider`, `key_fingerprint` (`sha256:` prefix + 16 hex chars), `key_hint` (masked, e.g. `sk-ant-a…8f2c`), `source_file`. **There is no plaintext field.**
- `reachability[]` — the provider `api_endpoints` and `auth_domain` each fingerprint can reach, derived from the static mapping. **These addresses were never pinged**; they are deductions from key shapes.
- `declared_egress[]` — domains declared in configs or skill instructions (with the source file); `observed_egress[]` — provider/declared hosts seen in existing local logs (the evidence is the log file itself).
- `content_surface[]` — static best-effort by tool kind: `cli` → prompts / code / env, `mcp_server` → prompts / env, `skill` → prompts; destinations are reachable endpoint hosts plus declared domains.
- `host_fingerprint` — a non-reversible hash of the hostname: traceable across archives, without publishing the machine name.

Credential shapes recognized today: Anthropic, OpenAI (including `sk-proj-` variants), OpenRouter, Google AIza, Groq, xAI, GitHub, GitLab, Hugging Face, Slack, NVIDIA, plus an entropy-gated OpenAI-compatible fallback (DeepSeek / Moonshot and other relays — reachability left empty).

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/alert-triangle.svg" width="22" align="absmiddle" alt=""/> Limitations

- **v0.1.0 implements `scan` only.** `report` / `scope` / `diff` are roadmap stubs: the commands exist, print a roadmap notice, and exit with code 2 — this README does not sell unimplemented capabilities.
- Read-only inventory: scanned binaries are never executed, so **tool versions are not detected**; skills are not run in a sandbox to observe behavior.
- `observed_egress` reads only existing local logs (the opencode data directory), filtered to known provider and declared hosts — runtime traffic capture belongs to a different tool class (egress proxies) and is deliberately out of scope.
- `content_surface` is static best-effort inference, not measured traffic.
- codex `config.toml` is scanned line-by-line with regexes, not fully TOML-parsed.
- Local LLM summarization (`--llm` via Ollama running qwen2.5-7b-instruct, fully offline) is on the roadmap — **not implemented and not runtime-verified in this release**; default runs are model-free.
- Windows is out of scope for v0.1 (macOS / Linux).

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/map.svg" width="22" align="absmiddle" alt=""/> Roadmap

- **m2 — scope + report**: `scope --provider anthropic` scopes the key fingerprints that must be rotated for one vendor (instead of "rotate all keys"); `report` renders the archivable admission report with its three tables — keys held / domains contacted / content that may leave — in markdown/HTML.
- **m3 — diff + signed archives**: `diff` compares two scans to expose silent skill updates and new credential drift; the report gains supply-chain review fields for admission audits; local ed25519 signing of the manifest.
- **Optional local LLM summarization**: `--llm` summarizes skill instructions through a local Ollama (qwen2.5-7b-instruct), fully offline; still an opt-in flag — default runs keep zero egress and zero model dependencies.
- More config paths and skill directory conventions.

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/coins.svg" width="22" align="absmiddle" alt=""/> Pricing

The scanner itself is MIT-licensed open source with no feature gates; trust is the product, and an inventory with paid-out views is not worth archiving. What is paid is the **team admission report pack** (self-hosted, delivered offline — no hosted SaaS, because air-gapping and data residency are the selling points):

| Item | Contents | Price |
|---|---|---|
| Open-source scanner | `scan` + the exposure-manifest v0 schema, MIT | Free |
| Team admission report pack | merged multi-host inventories, admission-review field templates, local ed25519-signed archives, monthly drift diffs | ¥18k / org / year (range ¥10-30k) |
| White-label for audit firms | per-project reporting | ¥3-5k / project |

The delivery and payment path is deliberately plain: an ed25519-signed license file (reusing the product's own signing mechanism) + email/WeChat delivery + bank-transfer invoicing; no account system, no third-party billing, no cloud component. The first paying customers come through a paid PoC — one real host, one real admission review filed with our report — and a yearly license afterwards. Reach out via [GitHub Issues](https://github.com/SuperMarioYL/agentmanifest/issues).

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/certificate.svg" width="22" align="absmiddle" alt=""/> License

[MIT](./LICENSE) © 2026 SuperMarioYL

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
