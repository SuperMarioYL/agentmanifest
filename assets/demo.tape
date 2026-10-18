# agentmanifest demo tape
#
# scripts/make_demo_cast.py executes every line starting with "$ " (in order,
# from the repository root) and records the combined output as an asciinema
# v2 cast at assets/demo.cast. Build the binary first:
#
#   go build -o agentmanifest ./cmd/agentmanifest
#   python3 scripts/make_demo_cast.py
#
# The recording runs against the shipped offline fixtures
# (testdata/fixtures) with a controlled environment, so every regeneration
# is deterministic. All credentials in the fixtures are fake placeholders.
$ ./agentmanifest version
$ ./agentmanifest scan --project testdata/fixtures --out .demo-out
$ head -36 .demo-out/exposure-manifest.yaml
