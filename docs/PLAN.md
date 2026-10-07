---
PLAN: "feat: actions — named commands on the whole list (Action, Actioner, ActionRunner)"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 4771191073984352337
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> Phase F6a of the network administration master plan (private repo `veltylabs/mjosefa-cms`; you do
> not need it). The renderer side (`webtyp.com/layout/crudview`) is a separate plan that runs after
> this one is published.

# Plan — `webtyp.com/view`: actions

**The spec is [docs/SPECS.md §9](SPECS.md#9-actions--commands-on-the-whole-list)** — read it
completely before writing code; this plan only orders the work. Also read §4 (why actions are not a
fourth capability), §7 (the caller adapter you extend) and §8 (the renderer conformance suite you
extend). Follow [AGENTS.md](../AGENTS.md).

## Development rules

- WASM/TinyGo package: only `webtyp.com/fmt` (no `errors`/`strings`/`strconv`/stdlib `fmt`), no
  `map`, no `reflect`.
- Asynchronous contract (§2): every outcome travels through `done`; never block; `done` nil is
  replaced by a no-op.
- No new capability wrapper structs (§4 / §9 "Why it is not a fourth capability"): `Actioner` is
  implemented once, on `*core`, and every presenter variant gets it by embedding.
- No string literals in logic; error messages exactly as in §9.
- Tests in `tests/` (package `tests`), runner `gotest ./...`. Never export a symbol only for tests.

## Design gate

**1. Prior art.**
- **Django admin "actions"**: named callables over the selected queryset, with an intermediate
  confirmation page. Adopted: named actions + optional confirmation; differs in that ours run over
  the whole loaded list (the case that motivated it: applying a plan).
- **React-Admin `bulkActionButtons` / custom toolbar actions**: buttons declared next to the list,
  each calling a data-provider method. Adopted: actions declared by the backend description (`Ops`),
  drawn by the renderer.
- **Terraform Cloud API**: `POST /runs/:id/actions/apply` — "apply" is an action on a resource, not
  an edit of it. Adopted: an action is an op call, distinct from Save.
- Why different: actions here are declared on the transport adapter (`Ops`), so the qualified op
  name is composed in one place (`Ops.qualified`), like List/Save/Update/Delete.

**2. Novice-name test.** `view.Action{Op, Label, Confirm, Args}`, `presenter.Actions()`,
`presenter.Run("apply_network", done)`, `Ops{…, Actions: []view.Action{…}}` — each reads as what it
does.

**3. Complexity ledger.**
```
Concepts the developer must learn   +1 (Action); Actioner/ActionRunner are renderer/lister-side
Files they must touch to do X       adding a button to a module's screen: 1 (its view.go)
Lines at the call site              one Action literal in Ops — ~5
Ways to do the same thing           0 (no way to run a non-CRUD op from a view exists today)
```

**4. Where it belongs.** The UI contract between modules and renderers is this package. The button
itself is the renderer's (`crudview`, next plan).

**5. What it deletes.** Nothing — new capability. It prevents the alternative (each module writing a
custom screen for one button).

## Stage 1 — `action.go` (new)

Declare `Action`, `Actioner`, `ActionRunner` exactly as §9 "Surface", with doc comments taken from
§9. Add the error constructor for unknown actions in the file that holds the other exact messages
(§5) — `view: Run: unknown action <op>`.

## Stage 2 — `presenter.go`

On `*core`:
- `Actions() []Action` — `lister.(ActionRunner)` → its `Actions()`; else `nil`.
- `Run(op string, done func(error))` — §9 "Behaviour" steps 1–5, using the records in `p.index`
  (in order) for `Args`.
Add `var _ Actioner = (*core)(nil)` and, in the existing compile-time checks block (or a new one in
`action.go`), one assertion per presenter variant (`*saveable`, `*crud`, …) that it satisfies
`Actioner` — proving every presenter has it.

## Stage 3 — `caller_lister.go`

- `Ops` gains `Actions []Action` (doc: §9).
- `NewCallerLister`: validate each action (`Op`, `Label` required — panic messages exactly as §9).
- `qualified()`: qualify every `Actions[i].Op` with the module, on a **copy** of the slice (never
  mutate the caller's `Ops`).
- `*callerLister` gets `Actions() []Action` (returns the actions with their **bare** op names, as the
  module declared them — `Run` is called with bare names) and
  `RunAction(op string, args model.Encodable, done func(error))` = find the action by bare name,
  `c.caller.Call(<qualified op>, args, nil, done)`; unknown → the §9 error through `done`.
  Every `caller*` wrapper embeds `*callerLister`, so all of them implement `ActionRunner` — add one
  compile-time assertion per wrapper.

## Stage 4 — conformance (`conformance/conformance.go`)

- `Driver` gains (doc comments in the same style as the existing fields):
  `ActionLabels func() []string` (labels of the action controls, in order),
  `ActionEnabled func(op string) bool`, `ClickAction func(op string)`,
  `ConfirmAction func()` (accepts the open confirmation; no-op when none is open).
- `FakeLister` gains `ActionList []view.Action` and `Ran []string` (ops run, in order) and
  implements `ActionRunner` (`RunAction` appends to `Ran` and calls `done(nil)`).
- New clauses (names exactly):
  - `actions_render_labels` — two actions → `ActionLabels()` equals their labels in order.
  - `actions_disabled_without_items` — empty list → `ActionEnabled(op)` false; after a reload with
    rows → true.
  - `action_without_confirm_runs_on_click` — `ClickAction` → `Ran == [op]`, then the list reloads.
  - `action_with_confirm_waits` — `Confirm` set: `ClickAction` → `Ran` empty; `ConfirmAction` →
    `Ran == [op]`.
  - `no_actions_without_runner` — a lister without `ActionRunner` → `ActionLabels()` empty.
- Update §8's clause list in SPECS.md with the five names.

## Stage 5 — tests (`tests/`)

`tests/action_test.go` (package-level, no renderer):
1. `view.New` over a lister without actions → `Actions()` empty; `Run("x", done)` → done gets
   `view: Run: unknown action x`.
2. With a fake `ActionRunner`: `Run` passes `Args(records)` built from the last `Reload`'s records in
   order; on success the presenter reloads (a counting lister proves `List` ran again); on error no
   reload happens and the error arrives through `done`.
3. `NewCallerLister` with `Ops{Module: "network_manager", List: "plan_network", Actions: []view.Action{{Op: "apply_network", Label: "Apply"}}}`
   over a recording fake `router.Caller`: `Run("apply_network", …)` calls
   `"network_manager.apply_network"`; the caller's `Ops` slice is unchanged afterwards.
4. Panics: empty `Op`, empty `Label` — exact messages.
5. Every presenter variant returned by `view.New` (8 lister shapes) satisfies `view.Actioner`.

## Stage 6 — docs

- `README.md`: add "Run a command on the whole list → `Ops.Actions` + `presenter.Run`" to its usage
  table with a 6-line example (`apply_network` with `Args` reading a fingerprint from the first record).
- SPECS.md: verify §9 against the code, then **remove its STATUS note**; §1 "Public surface" lists the
  three new types.

## Acceptance criteria

- `gotest ./...` green (including WASM).
- `grep -rn "type .*Actions.* struct" --include=*.go . | grep -v action.go` → empty (no wrapper structs).
- `grep -n "STATUS (remove" docs/SPECS.md` → empty.

| Stage | Files | Done when |
|---|---|---|
| 1 | `action.go` | types declared |
| 2 | `presenter.go` | `*core` implements `Actioner` |
| 3 | `caller_lister.go` | `Ops.Actions`, `ActionRunner` on every caller wrapper |
| 4 | `conformance/conformance.go`, SPECS §8 | 5 clauses |
| 5 | `tests/action_test.go` | green |
| 6 | `README.md`, SPECS §1/§9 | STATUS removed |
