---
PLAN: "feat: the tech-agnostic CRUD view contract — Descriptor + Presenter + conformance"
TAG: v0.1.0
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: **agents-workflow**.
> Orquestado por `tinywasm/app-releases/docs/REUSABLE_MODULES_MASTER_PLAN.md` — **Fase A3
> (compuerta)**. Doctrina: `tinywasm/app-releases/docs/CONSTRUCTION_HARNESS.md`.

# PLAN — `tinywasm/view`: el contrato de vista agnóstico de tecnología

Autocontenido, en español. Eres un agente **sin contexto previo** y **solo tienes este repo**
(`tinywasm/view`, hoy con el scaffold vacío de `gonew`). Todo contrato y justificación va inline.

---

## 1. Qué construye este paquete y por qué

Hoy la vista CRUD de un módulo de dominio **no vive en el módulo**: se escribe en el app
(`mjosefa-cms/modules/x/view.go`) importando `tinywasm/layout`. Eso tiene dos consecuencias que
rompen el arnés:

1. **El módulo no se puede testear en su propio repo.** La lógica de su vista —qué operación se
   dispara al cargar, qué hace seleccionar una fila, qué se envía al guardar/eliminar— solo se
   ejercita aguas abajo, en un app que **no tiene autoridad para arreglar** un defecto que
   encuentre (solo puede parchear). Es literalmente el bucle que `CONSTRUCTION_HARNESS.md` existe
   para romper.
2. **El módulo queda clavado a una tecnología de UI.** Para declarar su vista tiene que importar
   `layout`/`dom`/`form`. Cambiar de renderer (otro layout, un render nativo, HTMX) obligaría a
   tocar cada módulo.

`view` cierra ambas cosas con **una separación tipada**: el módulo declara un `Presenter` —el motor del ciclo CRUD, manejado solo por un `router.Caller` y un `model.Model`, **sin DOM ni formulario**—. Un renderer concreto (`layout/crudview`, otro plan) envuelve ese `Presenter` y lo dibuja como quiera.

El resultado: el módulo importa **solo `model` + `router` + `view`**, y su vista se prueba con un
`router.Caller` falso, sin navegador.

## 2. El patrón que justifica la forma: MVP / ViewModel

`view.Presenter` es el **Presenter** del patrón Model-View-Presenter (o el ViewModel de MVVM): la
lógica de interacción **agnóstica de UI**, separada de la View (el dibujo). Ese patrón existe por
exactamente la razón que necesitamos: el Presenter es **unit-testeable sin una UI**, porque no toca
widgets — solo datos y transporte. La View es "tonta": dibuja el estado del Presenter y le reenvía
los eventos del usuario. Cualquiera que sepa MVP reconoce la partición; quien no, la deduce de que
`Presenter` no importa `dom` ni `form`.

## 3. El contrato (paquete `view`, raíz)

`view` importa **solo** `model`, `router`, `json` y `fmt`. **Jamás** `dom`, `form`, `html`, ni ningún renderer. Ese es el invariante que lo hace reutilizable.

Para garantizar seguridad en tiempo de compilación y un flujo de datos explícito, el constructor del presentador recibe los componentes requeridos obligatorios directamente como argumentos posicionales, delegando configuraciones opcionales en opciones funcionales.

Las llamadas interactivas son completamente **síncronas** y aprovechan canales bloqueantes para acoplarse sobre el `Caller` asíncrono, reduciendo drásticamente la posibilidad de estados ilegales en la interfaz.

```go
package view

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

type Item struct {
	ID          string
	Label       string
	Description string
}

type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item
	Reload() error

	Selected() string
	Select(id string) model.Model

	CanSave() bool
	Save(payload model.Model) error

	CanDelete() bool
	Delete(id string) error
}

type Option func(*presenter)

func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option
func WithSaveOp(op string) Option
func WithDeleteOp(op string) Option
func WithArgs(args func() model.Encodable) Option
func WithFill(fill func(id string) model.Model) Option

func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.FielderSlice,
	project func(list model.FielderSlice) []Item,
	opts ...Option,
) Presenter
```

## 4. Por qué el form y el DOM se quedan FUERA de `view`

`Presenter.Save` envía el `payload model.Model` que recibe explícitamente; **no valida ni sincroniza el form**, porque `view` no puede importar `form`/`dom` sin clavar cada módulo a ese stack. La partición:

| Conocimiento | Dónde vive | Por qué |
|---|---|---|
| qué op, con qué payload, decodificar la lista, estado de selección | `view.Presenter` (agnóstico) | testeable con un Caller falso, sin DOM — **el objetivo** |
| generar inputs del schema, validar, sincronizar, dibujar filas, cablear clics | el **renderer** (`layout/crudview`, otro plan) | inherentemente específico de tecnología |

**La obligación del renderer** (guardar): antes de llamar `Presenter.Save`, sincroniza el form dentro de `Record` (y valida). Eso **no es un footgun silencioso**: `view/conformance` tiene una cláusula que **se pone roja** si el renderer envía datos sin sincronizar. Por doctrina, un "hay que acordarse de X" se cierra con un aviso ruidoso — aquí, un test rojo — no con prosa.

## 5. La frontera del codec: por qué `view` importa `json` y el módulo NO

Traducir la respuesta de `ListOp` (bytes JSON del wire) a los registros del dominio **requiere un
codec concreto**. El objetivo de toda la ola es que los **módulos de dominio** no importen `json`.
La resolución: **`view` (infraestructura) hace el paso bytes→modelo; el módulo solo aporta su tipo
de lista y una proyección pura.**

- `newList func() model.FielderSlice` — el módulo devuelve una lista fresca (`&CatalogItemList{}`).
  Fresca *por Reload* porque `json.Decode` sobre una lista **acumula**; reusar la instancia mezclaría
  recargas.
- `view.Presenter.Reload` hace: `list := newList(); json.Decode(raw, list); items := project(list)`.
- `project func(model.FielderSlice) []Item` — el módulo itera `list.Len()/At(i)`, hace type-assert a su registro concreto, construye los `Item` y llena su caché `id→registro`. Trabaja sobre **modelos tipados**, nunca sobre bytes ni `json`.

## 6. `view/mock` — el renderer de referencia (la prueba con forma de consumidor)

Regla del arnés: **una API no está publicada hasta que un test con forma de consumidor, DENTRO de la
librería, la prueba.** El consumidor de `view` es un *renderer*. Así que este repo trae un renderer
**headless** de referencia (sin DOM, basado en strings/estructuras) — el análogo de `router/mock` —
que demuestra que el `Presenter` es usable end-to-end y que sirve de referencia de lo que
`crudview` debe hacer.

`view/mock.Renderer` construido desde un `Presenter`:
- `Mount()` → llama `Presenter.Reload`.
- `Labels() []string` → los labels de `Presenter.Items()`.
- `Select(id)` → `Presenter.Select(id)` y "carga" el registro en un form headless (guarda los campos).
- `SetField(name, value)` → fija un campo del form headless.
- `Save()` → sincroniza el form headless dentro de `Presenter.Record()` y llama `Presenter.Save(payload)`.
- `Delete()` → `Presenter.Delete(Selected)`.

Reutiliza **`router/mock.Caller`** como transporte falso.

## 7. `view/conformance` — el arnés que prueban los renderers

El análogo exacto de `router/conformance`: exporta `conformance.Run(t, Factory)`; cualquier
renderer lo importa desde su `_test.go` y lo corre contra sí mismo. Un renderer que no lo pase, no
es un renderer.

```go
package conformance

import (
	"testing"
	"github.com/tinywasm/view"
)

type Factory struct {
	New func(t *testing.T, p view.Presenter) Driver
}

type Driver struct {
	Mount    func()
	Labels   func() []string
	Select   func(id string)
	SetField func(name, value string)
	Save     func()
	Delete   func()
}

func Run(t *testing.T, f Factory) { /* t.Run por cláusula */ }
```

Cláusulas (cada una un `t.Run`), manejadas con un `router/mock.Caller` que la suite mete en el `Presenter` y luego inspecciona:

1. **`mount_triggers_list_load`** — tras `Mount()`, el Caller recibió `ListOp`.
2. **`list_renders_item_labels`** — con la lista canned en `CannedResult`, `Mount()`; `Labels()` contiene los labels que `project` produjo.
3. **`select_fills_form`** — `Select(id)` y luego `Save()`; el registro enviado es el de ese id (via `Fill`). Prueba que seleccionar carga el form.
4. **`save_ships_form_values`** — `SetField("name","X")` y `Save()`; el registro en el wire lleva `name=="X"`. Esta es la cláusula que fuerza al renderer a sincronizar el form→Record antes de Save: si no lo hace, envía datos viejos y el test se pone rojo.
5. **`delete_ships_selected_record`** — `Select(id)` y `Delete()`; el Caller recibió `DeleteOp` con el registro.
6. **`no_save_capability_when_saveop_empty`** — con `SaveOp==""`, un renderer no debe ofrecer ni disparar guardar.

## 8. Reglas del arnés (obligatorio) y NO hacer

- **`view` no importa `dom`/`form`/`html`/`layout`/`mcp`/`unixid`.**
- **Cero `any` en la API pública** salvo el `model.Encodable`/`model.Model` que ya son el borde tipado. Cero `map` en firmas. Cero genéricos.
- **Reutiliza `router/mock.Caller`**.
- **No metas la lógica del form en `view`.**
- **Sin stdlib** en código que compila a WASM (`view` sí compila a wasm): usa `tinywasm/fmt`/`json`.
