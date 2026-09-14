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
