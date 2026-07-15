# view

Tech-agnostic CRUD view contract: a module declares its view options and presenters, and any renderer draws it.

## Design Goals

1. **Agnostic to UI Technology**: The domain module declares its intention, schema, operations, and list projections without importing any specific UI package (such as `dom`, `form`, or `html`). This allows changing renderers seamlessly (e.g. standard layout, HTMX, Native UI, SSR) without touching any business/domain module.
2. **Compile-time Safety**: Mandatory dependencies (such as the `Caller`, template `Record`, `ListOp`, `NewList`, and `Project` callbacks) are required as direct positional parameters of the constructor `view.New`. Purely optional features (such as `Title`, `SaveOp`, `DeleteOp`, `Fill`, `Args`, etc.) are configured via functional options. There are no exposed configuration structs where fields could be accidentally left unassigned.
3. **Synchronous Go Idiomatic Design**: `Reload`, `Save`, and `Delete` block synchronously, returning an `error`. This eliminates continuation-passing style (CPS) callbacks (`done func(error)`), avoiding dangling UI states and complex test synchronization. Internally, the asynchronous network caller is seamlessly wrapped with blocking channels.
4. **Explicit Form Synchronization**: The presenter's `Save(payload model.Model)` explicitly accepts the synchronized record, establishing an obvious, unidirectional data flow and eliminating the risk of mutations on shared pointers behind the scenes.

## API Reference

The complete `view` package API is defined as follows:

```go
package view

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Item is ONE projected row of the list — a neutral form that any renderer can draw.
type Item struct {
	ID          string // selection key
	Label       string // primary text
	Description string // secondary text (e.g. SKU, IP, subtitle)
}

// Presenter is the UI-agnostic engine behind any CRUD view.
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item                 // current decoded list from the last Reload()
	Reload() error                 // synchronously invokes ListOp and decodes it into Items

	Selected() string              // current selected ID ("" if none)
	Select(id string) model.Model  // marks id as current and returns its full model (via Fill)

	CanSave() bool                 // whether SaveOp is supported
	Save(payload model.Model) error // synchronously sends the explicit payload to SaveOp

	CanDelete() bool               // whether DeleteOp is supported
	Delete(id string) error        // synchronously sends the full record of id to DeleteOp
}

// Option represents a functional configuration option.
type Option func(*presenter)

func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option
func WithSaveOp(op string) Option
func WithDeleteOp(op string) Option
func WithArgs(args func() model.Encodable) Option
func WithFill(fill func(id string) model.Model) Option

// New constructs a compile-time safe Presenter with mandatory components.
func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.FielderSlice,
	project func(list model.FielderSlice) []Item,
	opts ...Option,
) Presenter
```

---

## Quick Start

### 1. Declaring a View (Domain Module)

```go
package catalog

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

func NewCatalogView(caller router.Caller) view.Presenter {
	record := &CatalogItem{}
	var cache = make(map[string]*CatalogItem)

	return view.New(
		caller,
		record,
		"catalog.list",
		func() model.FielderSlice { return &CatalogItemList{} },
		func(list model.FielderSlice) []view.Item {
			l := list.(*CatalogItemList)
			items := make([]view.Item, l.Len())
			for i := 0; i < l.Len(); i++ {
				item := l.At(i).(*CatalogItem)
				items[i] = view.Item{
					ID:          item.ID,
					Label:       item.Name,
					Description: item.SKU,
				}
				cache[item.ID] = item
			}
			return items
		},
		view.WithTitle("Catalog Management"),
		view.WithSaveOp("catalog.save"),
		view.WithFill(func(id string) model.Model {
			if id == "" {
				return nil
			}
			return cache[id]
		}),
	)
}
```

### 2. Wrapping with a Renderer (UI Layer)

Any concrete UI renderer (like `layout/crudview`) wraps the `Presenter`, generates form inputs from `Record().Schema()`, synchronizes user input back into `Record()` before calling `Save(payload)`, and renders the list items:

```go
package crudview

import (
	"github.com/tinywasm/view"
)

type Renderer struct {
	p view.Presenter
}

func (r *Renderer) OnSaveClicked() {
	// 1. Sync form inputs into the record
	rec := r.p.Record()
	r.syncFormToRecord(rec)

	// 2. Instruct presenter to save explicitly and synchronously
	if err := r.p.Save(rec); err != nil {
		r.ShowError(err)
	} else {
		r.ShowSuccess()
	}
}
```

## Reference and Conformance

* **`view/mock`**: Package containing a reference headless renderer (`mock.Renderer`) useful for browser-less simulations and unit testing.
* **`view/conformance`**: Test arnes package that exports `conformance.Run(t, Factory)`. Any prospective renderer must satisfy the 6 conformance clauses to be considered correct.
