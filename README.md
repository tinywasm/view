# tinywasm/view
<img src="docs/img/badges.svg">

Tech-agnostic CRUD view contract: a domain module declares its list, record and
operations; any renderer (DOM, HTMX, SSR, native, headless test) draws it.

This README is the **official usage document**. It is written so that an agent/LLM with
no prior context can create or edit a visual component correctly guided only by the
typed signatures below. If something here requires reading the implementation, that is
a defect — report it.

> Alignment note: this document describes the target API defined by the construction
> harness. `docs/PLAN.md` tracks the code's convergence to it (gated on
> `docs/PLAN_MODEL.md` being executed in `tinywasm/model` first).

## I want to… → use

| I want to… | Use |
|---|---|
| Create a list/detail view for my model | `view.New(caller, &X{}, "x.list", func() model.ModelSlice { return &XList{} }, opts...)` |
| Make my rows appear in the list | Implement `Item() view.Item` on the record type (`view.Itemizer`) |
| Enable saving | `view.WithSaveOp("x.save")` — the returned Presenter then satisfies `view.Saver` |
| Enable deleting | `view.WithDeleteOp("x.delete")` — the returned Presenter then satisfies `view.Deleter` |
| Know if the view can save/delete (renderer side) | `s, ok := p.(view.Saver)` / `d, ok := p.(view.Deleter)` |
| Load / refresh the list | `p.Reload()` (synchronous, returns `error`) |
| Pick a record and get its full model | `m := p.Select(id)` (`nil` if the id is unknown) |
| Clear the selection | `p.Deselect()` |
| Filter the list as the user types | `p.Filter(term)` (local, case-insensitive over Label+Description) |
| Send list arguments (pagination, remote search) | `view.WithArgs(func() model.Encodable { … })` then `p.Reload()` |
| Show an error/success message | Renderer's job: branch on the `error` returned by `Reload`/`Save`/`Delete` |
| Test a renderer implementation | `conformance.Run(t, factory)` — it must pass every clause |
| Simulate a view without a browser | `view/mock.Renderer` |

## API Reference

The complete public API of `view`:

```go
package view

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Item is ONE projected row of the list — the neutral form any renderer can draw.
// No markup: only what a list needs to display and let the user pick a record.
type Item struct {
	ID          string // selection key
	Label       string // primary text
	Description string // secondary text (a SKU, an IP, a subtitle)
}

// Itemizer is implemented by a domain record that knows how to project itself as a
// list row. It is the ONLY view-specific code a module writes on its model.
type Itemizer interface {
	Item() Item
}

// Presenter is the UI-agnostic core behind any CRUD view: list, select, reload.
// Always present. Save/Delete are separate capabilities (see Saver/Deleter).
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item             // projected list from the last Reload
	Filter(term string) []Item // local case-insensitive match over Label+Description; "" returns all
	Reload() error             // synchronously calls ListOp, decodes, projects and indexes

	Selected() string             // currently selected id ("" if none)
	Select(id string) model.Model // marks id and returns its record from the internal index; unknown id → nil, selection unchanged
	Deselect()                    // clears the selection
}

// Capabilities. The renderer discovers them by type assertion at the seam —
// the same pattern the composition root uses with router.APIModule.
// They are only present when the matching option was given to New:
// no WithSaveOp ⇒ the returned value has no Save method ⇒ p.(Saver) fails.
type Saver interface {
	Save(payload model.Model) error // synchronously ships the explicit payload to SaveOp
}
type Deleter interface {
	Delete(id string) error // ships the indexed record of id to DeleteOp; unknown id → error
}

// Option is a functional configuration option for New.
type Option func(*config)

func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option
func WithSaveOp(op string) Option
func WithDeleteOp(op string) Option
func WithArgs(args func() model.Encodable) Option

// New builds the presenter. Mandatory collaborators are positional (the compiler
// enforces their presence); a nil/empty mandatory value panics at construction —
// a loud development diagnostic, never a deferred runtime mystery.
func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.ModelSlice,
	opts ...Option,
) Presenter
```

## Quick Start

### 1. The domain module declares its view (3 steps)

```go
package catalog

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// Step 1 — the record projects itself as a list row.
func (c *CatalogItem) Item() view.Item {
	return view.Item{ID: c.ID, Label: c.Name, Description: c.SKU}
}

// Step 2 — build the presenter. No projection loop, no cache, no fill:
// the presenter iterates the decoded list and indexes id → model itself.
func NewCatalogView(caller router.Caller) view.Presenter {
	return view.New(
		caller,
		&CatalogItem{},
		"catalog.list",
		func() model.ModelSlice { return &CatalogItemList{} },
		view.WithTitle("Catalog Management"),
		view.WithSaveOp("catalog.save"), // Step 3 — declare capabilities you support
	)
}
```

That is the entire module-side code. `CatalogItem`/`CatalogItemList` are generated by
`ormc` and already satisfy `model.Model`/`model.ModelSlice`.

### 2. The renderer wraps the presenter (UI layer)

A renderer never imports the domain module. It draws `Items()`, generates form inputs
from `Record().Schema()`, and discovers capabilities by assertion:

```go
package crudview

import "github.com/tinywasm/view"

type Renderer struct{ p view.Presenter }

func (r *Renderer) Mount() {
	if err := r.p.Reload(); err != nil {
		r.ShowError(err) // messages are the renderer's concern
		return
	}
	r.drawList(r.p.Items())
	if _, ok := r.p.(view.Saver); ok {
		r.drawSaveButton() // only exists if the module declared WithSaveOp
	}
	if _, ok := r.p.(view.Deleter); ok {
		r.drawDeleteButton()
	}
}

func (r *Renderer) OnSaveClicked() {
	s := r.p.(view.Saver) // safe: the button only exists if the assertion held
	rec := r.p.Record()
	r.syncFormToRecord(rec) // explicit, unidirectional: form → record → Save
	if err := s.Save(rec); err != nil {
		r.ShowError(err)
		return
	}
	r.ShowSuccess()
}

func (r *Renderer) OnSearchTyped(term string) {
	r.drawList(r.p.Filter(term)) // local filtering; remote search = WithArgs + Reload
}
```

## Error model

- `Reload`, `Save`, `Delete` are **synchronous** and return `error`. The `error` return
  IS the user-message channel: the renderer decides how to present it (toast, inline,
  console). `view` never renders, logs, or swallows messages — there is no `SetLog`.
- `New` **panics** on nil/empty mandatory collaborators. These are programmer wiring
  bugs, detected deterministically at startup during development (the `template.Must`
  pattern). Logging and continuing would return a half-built presenter that crashes far
  from the cause — a deferred silent failure, which the harness forbids.
- Misuse that cannot be made a compile error is a loud error, never silence:
  `Delete` of an unknown id errors (nothing is sent), a row type that does not
  implement `Itemizer` makes `Reload` fail naming the offending type.

## Design Goals

1. **Agnostic to UI technology** — the module never imports `dom`, `form` or `html`;
   any renderer can draw the contract.
2. **Agnostic to codec and transport** — `view` imports only `model` and `router`;
   the transport's codec decodes into the module's typed list (`model.ModelSlice`).
3. **Compile-time safety** — mandatory collaborators are positional in `New`;
   capabilities are method sets, so calling `Save` on a view without `SaveOp` does not
   compile past the type assertion.
4. **Synchronous Go idiomatic design** — `Reload`/`Save`/`Delete` block and return
   `error`; the async network caller is wrapped internally with channels. No CPS
   callbacks, no dangling UI states.
5. **Explicit form synchronization** — `Save(payload)` takes the synchronized record
   explicitly: unidirectional data flow, no hidden shared-pointer mutations.
6. **Glue lives here, once** — projection loop, id→model index and capability wiring
   are implemented in this package, not repeated in every module.

## WebAssembly/TinyGo Compatibility

To ensure 100% compatibility with WebAssembly (WASM) and TinyGo targets, standard library packages (such as `fmt`, `encoding/json`, or `encoding/binary`) should be avoided in production code. Use the following tech-agnostic, low-allocation alternatives instead:
- `github.com/tinywasm/fmt` for formatting and error creation.
- `github.com/tinywasm/json` for JSON serialization/deserialization.
- `github.com/tinywasm/binary` for binary protocols.

## Reference and Conformance

- **`view/mock`** — headless reference renderer for browser-less simulation and unit
  tests.
- **`view/conformance`** — exports `conformance.Run(t, Factory)`. A renderer is correct
  only if it passes every clause (list load on mount, label rendering via `Itemizer`,
  select/deselect, save/delete capability assertions, filter semantics, loud errors on
  unknown ids).
- **`docs/PLAN.md`** — the harness-alignment refactor plan for this library.
- **`docs/PLAN_MODEL.md`** — the `model.ModelSlice` prerequisite, to be executed in
  `tinywasm/model`.
