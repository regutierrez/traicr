# Mutation testing

Traicr uses [Gremlins](https://github.com/go-gremlins/gremlins) `v0.6.0`. It is pinned in `scripts/mutation.sh`; the script installs that version into the user's cache rather than changing `go.mod`. Gremlins was chosen over `go-mutesting` because it is maintained, can restrict mutations to a Git diff, writes JSON reports, and clearly distinguishes lived, uncovered, timed-out, and non-compiling mutations.

Mutation testing complements the ordinary tests. It does not replace them, and a score by itself does not establish correctness.

## Commands

Run mutations on code changed from a Git ref:

```sh
./scripts/mutation.sh changed origin/main artifacts/mutation-changed.json
```

Run the complete module baseline:

```sh
./scripts/mutation.sh full artifacts/mutation-full.json
```

Run one package directory while investigating a survivor:

```sh
./scripts/mutation.sh scope ./internal/archive artifacts/mutation-archive.json
```

`GREMLINS_WORKERS` defaults to `2`. `GREMLINS_TIMEOUT_COEFFICIENT` defaults to `20`; Gremlins multiplies package coverage-test time by this value to determine each mutant's timeout.

The pull-request workflow mutates only changed Go lines relative to the PR base. The scheduled Monday run and manual workflow dispatch mutate the complete module. Both retain the JSON report as a workflow artifact. No efficacy or coverage threshold is configured yet. Tool failures still fail the job, but lived mutants do not: reviewers must inspect and classify them until enough reviewed baselines exist to choose a useful non-regression threshold.

## Reading a report

- `KILLED`: a test failed after the mutation, as intended.
- `LIVED`: the tests allowed changed behavior. Add a behavior test, remove unnecessary production logic, or document why the mutation is equivalent.
- `NOT COVERED`: no test reached the mutated expression. Decide whether that path is required before adding a test.
- `TIMED OUT`: inconclusive. Re-run the scope without competing workloads and inspect whether the mutation created an infinite or unusually slow path.
- `NOT VIABLE`: the mutation did not compile. It does not demonstrate test strength.

Do not suppress a survivor merely to improve the percentage. Permanent checks belong in ordinary `*_test.go` files.

## Initial focused run

The initial run was made on 2026-09-06 with Gremlins `v0.6.0` and two workers. Archive and authentication used timeout coefficient `10`; search was rerun serially with coefficient `20` after a concurrent run timed out under load. It used the current archive, store search, and HTTP authentication tests. `internal/domain/types.go` contains declarations but no mutable decision expressions, so its dry run produced no mutants.

| Scope | Killed | Lived | Not covered | Timed out | Classification |
| --- | ---: | ---: | ---: | ---: | --- |
| `internal/archive` | 118 | 40 | 13 | 5 | Open test gaps and inconclusive timeouts |
| `internal/server/auth.go` | 30 | 5 | 2 | 0 | Open authentication and CSRF test gaps |
| `internal/store/search.go` | 37 | 5 | 0 | 1 | Open pagination and exact-query boundary gaps; one timeout |

These figures are an unreviewed starting point, not an accepted baseline or release gate.

### Survivor classification

All line numbers below refer to the source used for that run.

- **Archive splitting and metadata defaults — missing behavior checks:** `archive.go:97`, `archive.go:104`, `archive.go:126`, and `archive.go:133`. Tests do not distinguish the exact zero split boundary, generated collection time, exact split-size equality, or archive sequence arithmetic.
- **Archive validation limits — missing boundary checks:** `validate.go:40`, `validate.go:43`, `validate.go:53`, `validate.go:65`, `validate.go:119`, `validate.go:146`, `validate.go:153`, `validate.go:161`, `validate.go:165`, `validate.go:175`, `validate.go:178`, `validate.go:179`, `validate.go:202`, `validate.go:205`, `validate.go:212`, `validate.go:215`, `validate.go:218`, `validate.go:221`, and `validate.go:227`. Existing tests reject representative invalid archives but do not distinguish every exact configured limit, path-depth edge, metadata byte edge, ZIP directory bound, and central-directory field offset changed by these mutants.
- **ZIP64 preflight — not covered:** `validate.go:187`, `validate.go:191`, and `validate.go:195`. Coverage did not reach malformed ZIP64 locator, offset, and header branches. These need small malformed ZIP64 fixtures, not assertions against implementation details.
- **Archive timeouts — inconclusive:** `archive.go:117`, `archive.go:121`, `archive.go:126`, `archive.go:225`. These mutations alter loop progress or termination and timed out. Re-run them in isolation before deciding whether they are killed by timeout or expose slow tests. The timeout recorded at `archive.go:117` also appeared as a killed mutant of another type, so status is mutation-specific.
- **Session expiry — missing security boundary check:** `auth.go:52`. Changing `now >= expires` to `now > expires` accepted a cookie during its exact expiry second.
- **Origin fallback — missing CSRF checks:** `auth.go:89` and `auth.go:91`. Negating the empty-Origin, successful Referer parse, and non-empty Referer host branches survived. Add requests covering a valid same-origin Referer and malformed or hostless Referers when no Origin header is present.
- **Expired login response — missing behavior check:** `auth.go:129`. Tests do not distinguish the `login_expired` response from an earlier form-validation response when the login cookie is absent.
- **Referer construction — not covered:** `auth.go:92`. Arithmetic mutations in `scheme + "://" + host` were not reached because the Referer fallback is uncovered.
- **Search page boundaries — missing pagination checks:** `search.go:94` and `search.go:101`. Tests do not distinguish fetching exactly `limit+1` rows from nearby arithmetic or setting a cursor only when an additional result exists.
- **Exact-search index thresholds — missing behavior checks:** `search.go:123`. Mutations at the 3-byte and 1,024-byte trigram boundaries survived. Tests should prove short queries fall back to scanning and both threshold edges return the same correct results.
- **Exact-search timeout — inconclusive:** `search.go:123`. Negating the 1,024-byte upper-bound check timed out once even after a serial rerun with coefficient `20`; investigate separately before classifying its behavior.

No survivor from this focused run has been classified as equivalent or as unnecessary production logic. That classification requires inspecting the concrete generated change and proving the observable behavior is unchanged; the JSON report records location and mutator but not the mutated source diff.
