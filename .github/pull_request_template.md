## Summary

What this changes and why. Reference the design spec (AGENTS.md) when
behavior deviates from it.

## Test checklist

- [ ] `gofmt -l .` clean
- [ ] `go vet ./...` and `go vet -tags live ./...` pass
- [ ] `go test -count=1 ./...` passes
- [ ] `bash skills_test.sh` passes
- [ ] `node cli/assets/oma.test.js` passes
- [ ] Tier 1 (`mise run live`) run when the runtime, bridge template or
      bundler changed
- [ ] Docs/skills updated when behavior changed

## Related issues

Fixes #
