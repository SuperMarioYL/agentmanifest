# 暴露清单样例(exposure-manifest sample)

本文件是 `agentmanifest scan` 对仓库自带离线 fixtures(testdata/fixtures)真实运行产出的
exposure-manifest.yaml 原文,未做任何手工修饰。fixtures 中的所有密钥都是伪造的供应商格式占位符,
清单里也只包含 SHA-256 指纹与掩码提示,不含任何明文。

运行环境:macOS(darwin/arm64),agentmanifest 0.1.0,manifest schema v0。

复现命令:

```console
$ go build -o agentmanifest ./cmd/agentmanifest
$ HOME=$PWD/testdata/fixtures/home GROQ_API_KEY=gsk_... \
  ./agentmanifest scan --project $PWD/testdata/fixtures --out .demo-out
scanned host host_d22ee280bd58feb6: 10 tools, 8 credentials (sha256 fingerprints only), 8 key-to-provider reachability rows, 4 declared egress domains, 2 observed
wrote .demo-out/exposure-manifest.yaml
wrote .demo-out/exposure-manifest.json
```

清单按"持有什么 → 能到哪 → 配置联系谁 → 可能送出什么"四层组织每个工具:

- `credentials`:工具持有的密钥,仅 SHA-256 指纹 + 掩码提示 + 来源文件;
- `reachability`:由"密钥形态 → 厂商 → API endpoint"静态映射推导,未发送任何探测包;
- `declared_egress` / `observed_egress`:配置与 skill 指令中声明的域名,以及本机既有日志中出现过
  的已知厂商域名(只读既有文件,不做主动联网探测);
- `content_surface`:按工具类别静态推断可能外发的内容类别(prompts / code / env)及其目的地。

完整 YAML 输出如下:

```yaml
schema_version: v0
generated_at: 2026-10-18T23:31:24.806677Z
host_fingerprint: host_d22ee280bd58feb6
tools:
    - name: claude-code
      kind: cli
      install_path: ~/.claude
      credentials:
        - provider: anthropic
          key_fingerprint: sha256:9d362ee48b6717c8
          key_hint: sk-ant-a…MPLE
          source_file: ~/.claude/settings.json
      reachability:
        - key_fingerprint: sha256:9d362ee48b6717c8
          provider: anthropic
          api_endpoints:
            - https://api.anthropic.com/v1
          auth_domain: console.anthropic.com
      content_surface:
        - data_class: prompts
          destinations:
            - api.anthropic.com
        - data_class: code
          destinations:
            - api.anthropic.com
        - data_class: env
          destinations:
            - api.anthropic.com
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
      content_surface:
        - data_class: prompts
          destinations:
            - api.openai.com
            - relay.internal.example
        - data_class: code
          destinations:
            - api.openai.com
            - relay.internal.example
        - data_class: env
          destinations:
            - api.openai.com
            - relay.internal.example
    - name: host-environment
      kind: cli
      install_path: env
      credentials:
        - provider: groq
          key_fingerprint: sha256:e5766bcf10184091
          key_hint: gsk_FAKE…mnop
          source_file: env:GROQ_API_KEY
      reachability:
        - key_fingerprint: sha256:e5766bcf10184091
          provider: groq
          api_endpoints:
            - https://api.groq.com/openai/v1
          auth_domain: console.groq.com
      content_surface:
        - data_class: prompts
          destinations:
            - api.groq.com
        - data_class: code
          destinations:
            - api.groq.com
        - data_class: env
          destinations:
            - api.groq.com
    - name: opencode
      kind: cli
      install_path: ~/.config/opencode
      credentials:
        - provider: openrouter
          key_fingerprint: sha256:9f74aa5f94822d47
          key_hint: sk-or-v1…beef
          source_file: ~/.config/opencode/opencode.json
        - provider: anthropic
          key_fingerprint: sha256:557fa01c9a18683d
          key_hint: sk-ant-a…MPLE
          source_file: <project>/testdata/fixtures/opencode.json
        - provider: openai
          key_fingerprint: sha256:e5f3e09ccc55504e
          key_hint: sk-proj-…MPLE
          source_file: ~/.local/share/opencode/auth.json
      reachability:
        - key_fingerprint: sha256:9f74aa5f94822d47
          provider: openrouter
          api_endpoints:
            - https://openrouter.ai/api/v1
          auth_domain: openrouter.ai
        - key_fingerprint: sha256:557fa01c9a18683d
          provider: anthropic
          api_endpoints:
            - https://api.anthropic.com/v1
          auth_domain: console.anthropic.com
        - key_fingerprint: sha256:e5f3e09ccc55504e
          provider: openai
          api_endpoints:
            - https://api.openai.com/v1
          auth_domain: platform.openai.com
      observed_egress:
        - domain: api.anthropic.com
          evidence_source: ~/.local/share/opencode/log/app.log
        - domain: api.openai.com
          evidence_source: ~/.local/share/opencode/log/app.log
      content_surface:
        - data_class: prompts
          destinations:
            - api.anthropic.com
            - api.openai.com
            - openrouter.ai
        - data_class: code
          destinations:
            - api.anthropic.com
            - api.openai.com
            - openrouter.ai
        - data_class: env
          destinations:
            - api.anthropic.com
            - api.openai.com
            - openrouter.ai
    - name: claude-code:brave-search
      kind: mcp_server
      install_path: <project>/testdata/fixtures/.mcp.json
      credentials:
        - provider: github
          key_fingerprint: sha256:7fb167a31ca240ec
          key_hint: github_p…6789
          source_file: <project>/testdata/fixtures/.mcp.json
      reachability:
        - key_fingerprint: sha256:7fb167a31ca240ec
          provider: github
          api_endpoints:
            - https://api.github.com
          auth_domain: github.com
      content_surface:
        - data_class: prompts
          destinations:
            - api.github.com
        - data_class: env
          destinations:
            - api.github.com
    - name: claude-code:docs
      kind: mcp_server
      install_path: ~/.claude.json
      content_surface:
        - data_class: prompts
        - data_class: env
    - name: opencode:context7
      kind: mcp_server
      install_path: ~/.config/opencode/opencode.json
      content_surface:
        - data_class: prompts
        - data_class: env
    - name: opencode:fetch
      kind: mcp_server
      install_path: <project>/testdata/fixtures/opencode.json
      content_surface:
        - data_class: prompts
        - data_class: env
    - name: release-notes
      kind: skill
      install_path: <project>/.claude/skills/release-notes
      declared_egress:
        - domain: api.alphadocs.example
          source: release-notes/SKILL.md
      content_surface:
        - data_class: prompts
          destinations:
            - api.alphadocs.example
    - name: web-reporter
      kind: skill
      install_path: ~/.claude/skills/web-reporter
      credentials:
        - provider: huggingface
          key_fingerprint: sha256:c47c3eadac1e80f3
          key_hint: hf_FAKE0…2345
          source_file: web-reporter/config.json
      reachability:
        - key_fingerprint: sha256:c47c3eadac1e80f3
          provider: huggingface
          api_endpoints:
            - https://router.huggingface.co/v1
          auth_domain: huggingface.co
      declared_egress:
        - domain: reporter.alphadocs.example
          source: web-reporter/SKILL.md
      content_surface:
        - data_class: prompts
          destinations:
            - reporter.alphadocs.example
            - router.huggingface.co
```

字段定义见 [schemas/exposure-manifest.v0.json](../../schemas/exposure-manifest.v0.json);
本次运行的记录(命令、结果计数、已知限制)见 [docs/demo-results.json](../demo-results.json)。
