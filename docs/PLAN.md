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

`view` cierra ambas cosas con **una separación tipada**: el módulo declara un `Descriptor` (datos:
su registro, sus operaciones, cómo proyectar la lista) sin nombrar **ninguna** tecnología de UI, y
`view.New` construye un `Presenter` —el motor del ciclo CRUD, manejado solo por un `router.Caller`
y un `model.Model`, **sin DOM ni formulario**—. Un renderer concreto (`layout/crudview`, otro plan)
envuelve ese `Presenter` y lo dibuja como quiera.

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

`view` importa **solo** `model`, `router`, `json` (ver §5, la frontera del codec) y `fmt`. **Jamás**
`dom`, `form`, `html`, ni ningún renderer. Ese es el invariante que lo hace reutilizable.

```go
package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Item es UNA fila proyectada de la lista — la forma neutra desde la que cualquier renderer
// dibuja una tarjeta/fila. No lleva markup: solo lo que una lista necesita para mostrar y dejar
// elegir un registro.
type Item struct {
	ID          string // clave de selección
	Label       string // texto principal
	Description string // texto secundario (un SKU, una IP, un subtítulo)
}

// Descriptor es lo que un módulo DECLARA sobre su vista CRUD. Es DATOS, no comportamiento:
// New construye el comportamiento (un Presenter) a partir de esto.
type Descriptor struct {
	Title  string      // encabezado; dónde ponerlo es cosa del renderer
	Record model.Model // fuente del form Y payload de save/delete (el renderer genera inputs de su schema)

	Caller router.Caller // el transporte, inyectado. El Descriptor NO nombra tecnología de transporte.

	ListOp   string // requerido: la op que Reload invoca para traer filas
	SaveOp   string // "" ⇒ Presenter.CanSave()==false (no se ofrece guardar)
	DeleteOp string // "" ⇒ Presenter.CanDelete()==false (no se ofrece eliminar)

	Args    func() model.Encodable            // payload de ListOp; nil = sin args
	NewList func() model.FielderSlice          // tipo de lista FRESCO por cada Reload (ver §5)
	Project func(list model.FielderSlice) []Item // módulo: FielderSlice decodificada → filas (+ su caché id→registro)
	Fill    func(id string) model.Model        // registro completo de id (típicamente de la caché de Project); nil = limpiar form

	SearchPlaceholder string
}

// Validate reporta los errores de configuración de los que un Presenter no puede recuperarse.
// New lo llama; falla ruidoso en construcción, no en el primer uso.
func (d Descriptor) Validate() error {
	if d.Caller == nil {
		return fmt.Err("view: Descriptor.Caller is required")
	}
	if d.ListOp == "" {
		return fmt.Err("view: Descriptor.ListOp is required")
	}
	if model.IsNil(d.Record) {
		return fmt.Err("view: Descriptor.Record is required")
	}
	if d.NewList == nil || d.Project == nil {
		return fmt.Err("view: Descriptor.NewList and Project are required")
	}
	return nil
}

// Presenter es el motor agnóstico de tecnología detrás de cualquier vista CRUD: listar,
// seleccionar, guardar, eliminar — manejado SOLO por el Caller y el model.Model del Descriptor.
// Sin DOM, sin form. Un renderer lo envuelve, dibuja su estado (Items, Selected) y llama sus
// métodos cuando el usuario interactúa (clic en fila → Select; clic en guardar → Save; …).
type Presenter interface {
	Items() []Item                 // la lista decodificada actual (resultado del último Reload)
	Reload(done func(error))       // invoca ListOp y decodifica en Items; done recibe el error (nil = ok)

	Selected() string              // el id que Select fijó por última vez ("" si ninguno)
	Select(id string) model.Model  // marca id como actual y devuelve su registro (via Fill); id=="" limpia y devuelve nil

	CanSave() bool                 // ¿el Descriptor ofreció SaveOp?
	Save(done func(error))         // envía Descriptor.Record a SaveOp (ver §4: el renderer sincroniza el form ANTES)

	CanDelete() bool               // ¿el Descriptor ofreció DeleteOp?
	Delete(id string, done func(error)) // envía el registro completo de id (via Fill) a DeleteOp
}

// New construye el Presenter de un Descriptor. Es la ÚNICA forma de construirlo: un renderer
// envuelve lo que New devuelve, no reimplementa este motor.
func New(d Descriptor) (Presenter, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &presenter{d: d}, nil // impl no exportada: superficie mínima (principio 5)
}
```

### Por qué CADA decisión

- **`Descriptor` es datos, no un builder con callbacks importados de `layout`.** Si el módulo
  construyera un `crudview.Config` (como hoy), importaría `layout` — y volvería a estar clavado a
  esa UI. Datos puros = el módulo declara *intención* sin nombrar tecnología.
- **`Presenter` es una interfaz, no un struct concreto.** El renderer y los tests dependen del
  contrato, no de la implementación; y `New` puede esconder el struct (superficie mínima). Hay una
  sola impl, pero el seam limpio importa más que ahorrar una interfaz.
- **`Record model.Model` es compartido.** El renderer genera el form de su `Schema()` y sincroniza
  el input del usuario DENTRO de él; el Presenter lo envía por el wire. Es el único punto de
  contacto entre "dibujar el form" (renderer) y "enviar el registro" (Presenter) — y es un tipo de
  `model`, no de una UI.
- **`Caller router.Caller` inyectado.** El Presenter invoca ops por nombre lógico sin conocer el
  wire; el app decide si es `mcp.NewCaller`, un `router/mock.Caller`, u otro. El módulo nunca
  importa `mcp`.
- **`SaveOp`/`DeleteOp == ""` ⇒ capacidad ausente.** Cerrado por defecto y greppable: un módulo de
  solo-lectura no ofrece guardar por *no escribir* el op, no por un flag que alguien olvida.

## 4. Por qué el form y el DOM se quedan FUERA de `view`

`Presenter.Save` envía `Descriptor.Record` tal cual; **no valida ni sincroniza el form**, porque
`view` no puede importar `form`/`dom` sin clavar cada módulo a ese stack. La partición:

| Conocimiento | Dónde vive | Por qué |
|---|---|---|
| qué op, con qué payload, decodificar la lista, estado de selección | `view.Presenter` (agnóstico) | testeable con un Caller falso, sin DOM — **el objetivo** |
| generar inputs del schema, validar, sincronizar, dibujar filas, cablear clics | el **renderer** (`layout/crudview`, otro plan) | inherentemente específico de tecnología |

**La obligación del renderer** (guardar): antes de llamar `Presenter.Save`, sincroniza el form
dentro de `Record` (y valida). Eso **no es un footgun silencioso**: `view/conformance` (§6) tiene
una cláusula que **se pone roja** si el renderer envía datos sin sincronizar. Por doctrina, un "hay
que acordarse de X" se cierra con un aviso ruidoso — aquí, un test rojo — no con prosa.

## 5. La frontera del codec: por qué `view` importa `json` y el módulo NO

Traducir la respuesta de `ListOp` (bytes JSON del wire) a los registros del dominio **requiere un
codec concreto**. El objetivo de toda la ola es que los **módulos de dominio** no importen `json`.
La resolución: **`view` (infraestructura) hace el paso bytes→modelo; el módulo solo aporta su tipo
de lista y una proyección pura.**

- `Descriptor.NewList func() model.FielderSlice` — el módulo devuelve una lista fresca (`&CatalogItemList{}`).
  Fresca *por Reload* porque `json.Decode` sobre una lista **acumula**; reusar la instancia mezclaría
  recargas (bug real ya visto en `mjosefa-cms`).
- `view.Presenter.Reload` hace: `list := d.NewList(); json.Decode(raw, list); items := d.Project(list)`.
- `Descriptor.Project func(model.FielderSlice) []Item` — el módulo itera `list.Len()/At(i)`,
  hace type-assert a su registro concreto, construye los `Item` y llena su caché `id→registro`
  (la misma que sirve `Fill`). Trabaja sobre **modelos tipados**, nunca sobre bytes ni `json`.

Así el **módulo importa `model`, no `json`** (el objetivo), y la dependencia inevitable del codec
queda en infraestructura (`view`), que sí puede nombrar `json` porque no es un módulo de dominio —
es la pieza que traduce el wire. Es la misma doctrina por la que `mcp`/`httpd` sí usan `json`.

> **Rechazado:** `Descriptor.Decode func(raw []byte) ([]Item, error)` (como hoy en `crudview`). Es
> más corto, pero **obliga al módulo a importar `json`** para el `json.Decode(raw, &list)` interno —
> reintroduce exactamente lo que esta ola elimina. `NewList`+`Project` cuesta ~1 línea más y deja al
> módulo codec-free.

## 6. `view/mock` — el renderer de referencia (la prueba con forma de consumidor)

Regla del arnés: **una API no está publicada hasta que un test con forma de consumidor, DENTRO de la
librería, la prueba.** El consumidor de `view` es un *renderer*. Así que este repo trae un renderer
**headless** de referencia (sin DOM, basado en strings/estructuras) — el análogo de `router/mock` —
que demuestra que el `Presenter` es usable end-to-end y que sirve de referencia de lo que
`crudview` debe hacer.

`view/mock.Renderer` construido desde un `Descriptor`:
- `Mount()` → llama `Presenter.Reload`.
- `Labels() []string` → los labels de `Presenter.Items()`.
- `Select(id)` → `Presenter.Select(id)` y "carga" el registro en un form headless (guarda los campos).
- `SetField(name, value)` → fija un campo del form headless.
- `Save()` → sincroniza el form headless dentro de `Descriptor.Record` y llama `Presenter.Save`.
- `Delete()` → `Presenter.Delete(Selected)`.

Reutiliza **`router/mock.Caller`** (ya existe: `Call`/`Dispatch`, graba `.Calls`, `CannedResult`/
`CannedError`) como transporte falso. **No inventes otro fake de Caller** (regla lego).

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

// Factory construye el renderer bajo prueba alrededor de desc y devuelve un Driver que simula la
// interacción del usuario con él. Es el seam específico de tecnología (como ServeFunc en router).
type Factory struct {
	New func(t *testing.T, desc view.Descriptor) Driver
}

// Driver simula los eventos de UI sobre el renderer, sin que la suite conozca su tecnología.
type Driver struct {
	Mount    func()                    // provoca el init: el renderer carga la lista
	Labels   func() []string           // lo que la lista muestra ahora
	Select   func(id string)           // simula seleccionar la fila con ese id
	SetField func(name, value string)  // fija un campo del form
	Save     func()                    // simula la acción guardar
	Delete   func()                    // simula la acción eliminar
}

func Run(t *testing.T, f Factory) { /* t.Run por cláusula */ }
```

Cláusulas (cada una un `t.Run`), manejadas con un `router/mock.Caller` que la suite mete en el
`Descriptor` y luego inspecciona:

1. **`mount_triggers_list_load`** — tras `Mount()`, el Caller recibió `ListOp`. (Mata el bug
   METHOD_NOT_FOUND por construcción: si el renderer no dispara la op correcta, rojo.)
2. **`list_renders_item_labels`** — con la lista canned en `CannedResult`, `Mount()`; `Labels()`
   contiene los labels que `Project` produjo.
3. **`select_fills_form`** — `Select(id)` y luego `Save()`; el registro enviado es el de ese id
   (via `Fill`). Prueba que seleccionar carga el form.
4. **`save_ships_form_values`** — `SetField("name","X")` y `Save()`; el registro en el wire lleva
   `name=="X"`. **Esta es la cláusula que fuerza al renderer a sincronizar el form→Record** antes de
   Save (§4): si no lo hace, envía datos viejos y el test se pone rojo.
5. **`delete_ships_selected_record`** — `Select(id)` y `Delete()`; el Caller recibió `DeleteOp` con
   el registro.
6. **`no_save_capability_when_saveop_empty`** — con `SaveOp==""`, un renderer no debe ofrecer ni
   disparar guardar (Driver.Save es no-op / el Caller nunca recibe un save).

**Por qué el Driver y no un render-a-string comparado literal:** la salida de UI es específica de
tecnología (un `*dom.Element` en crudview, HTML en SSR, nodos en React). Lo **universal y testeable**
son las interacciones con el `Caller` (qué ops se disparan, con qué payload) y los labels visibles.
El `Driver` expone justo esos disparadores — igual que `router/conformance` pide un `ServeFunc` que
maneja una petición sin que la suite sepa el transporte.

## 8. Reglas del arnés (obligatorio) y NO hacer

- **`view` no importa `dom`/`form`/`html`/`layout`/`mcp`/`unixid`.** Criterio duro:
  `grep -rn "tinywasm/\(dom\|form\|html\|layout\|mcp\|unixid\)" .` (excluyendo tests que ejerciten
  el mock headless, que tampoco los necesita) **vacío**.
- **Cero `any` en la API pública** salvo el `model.Encodable`/`model.Model` que ya son el borde
  tipado. Cero `map` en firmas. Cero genéricos.
- **Reutiliza `router/mock.Caller`**; no declares otro fake de Caller (regla lego).
- **No metas la lógica del form en `view`.** Si una cláusula necesita el form, va en el renderer y su
  obligación se verifica desde `view/conformance` vía el `Driver`.
- **Sin stdlib** en código que compila a WASM (`view` sí compila a wasm): usa `tinywasm/fmt`/`json`.
- Solo `gotest`. `codejob`/`gopush` no se ejecutan desde el plan.

## 9. Criterios de aceptación

1. `view` exporta `Item`, `Descriptor` (con `Validate`), `Presenter` (interfaz) y `New`, con la forma
   de §3.
2. `view.New` falla (error, no panic) si faltan `Caller`/`ListOp`/`Record`/`NewList`/`Project`.
3. `view/mock.Renderer` existe, se construye de un `Descriptor`, y **pasa `view/conformance`** (test
   en `tests/`), usando `router/mock.Caller`.
4. Un test con forma de **módulo** (sin renderer): construye un `Descriptor` con un `router/mock.Caller`,
   `p, _ := view.New(desc)`, y verifica que `Reload`→`ListOp`, `Save`→`SaveOp` con el `Record`,
   `Delete`→`DeleteOp` con el registro de `Fill`, `Select("")`→nil. **Sin DOM.**
5. `grep` de imports de UI/transporte/unixid en `view` (no-test) vacío (§8).
6. `gotest ./...` verde en stdlib **y** wasm (`view` compila a ambos).

## 10. Etapas

| # | Etapa | Archivos | Criterio |
|---|---|---|---|
| 1 | Contrato | `view.go` (`Item`, `Descriptor`, `Validate`, `Presenter`, `New`) + `presenter.go` (impl no exportada) | compila; `New` valida |
| 2 | Frontera codec | `Reload` decodifica via `NewList`+`json.Decode`+`Project` | módulo codec-free por diseño |
| 3 | Renderer de referencia | `mock/renderer.go` (headless, reusa `router/mock.Caller`) | construible desde `Descriptor` |
| 4 | Conformance | `conformance/conformance.go` (Factory+Driver+6 cláusulas) | exporta `Run` |
| 5 | Pruebas | `tests/conformance_test.go` (mock pasa conformance) + `tests/module_test.go` (forma-módulo, sin DOM) | `gotest ./...` verde (stdlib+wasm) |
| 6 | Docs | `README.md` (Quick Start: un módulo declara un `Descriptor`; un renderer lo dibuja) | refleja el contrato |
