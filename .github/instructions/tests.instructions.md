---
applyTo: "**/*_test.go"
---

# Test Review Rules

- Tests use the standard `testing` package. No third-party assertion libraries (no testify, no gomega).
- Property-based tests use `pgregory.net/rapid`. Name them `TestProperty[N]_DescriptiveName` or `TestProperty_DescriptiveName`.
- Custom rapid generators return `*rapid.Generator[T]` and are named `xxxGen()`.
- Every test package that touches the filesystem must use `t.TempDir()` for isolation. Never use hardcoded paths or the real `data/` directory.
- Use `t.Helper()` in test helper functions so failure locations point to the caller.
- Use `t.Setenv()` instead of `os.Setenv()` so environment is restored after the test.
- Use `t.Parallel()` where tests are independent and don't share mutable state.
- Discard logger: each package provides `nopLogger()` returning `slog.New(slog.NewTextHandler(io.Discard, nil))`.
- The `auth` package lowers `bcryptCost` in `TestMain`. If adding a new test package that uses bcrypt, follow the same pattern.
- Rapid test failure files are stored in `testdata/rapid/` and must be committed so regressions are reproducible.
- Do not use `time.Sleep` for synchronization except when testing token expiry with very short TTLs.
