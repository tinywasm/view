# PLAN — Refactor de `tinywasm/view` al patrón de construcción arnés/lego

> Objetivo: que el API de `view` cumpla `CONSTRUCTION_HARNESS.md` — tipado, explícito,
> imposible de usar mal. El README de este repo ya describe el **estado objetivo**; el
> código debe converger a él. Este plan es la lista ordenada de cambios.

## Dependencia previa (bloqueante)

**No ejecutar este plan hasta que `tinywasm/model` publique `model.ModelSlice`**
(ver `docs/PLAN_MODEL.md`, que se ejecuta en el repo de `model`). Una vez publicado:

```bash
go get github.com/tinywasm/model@<v0.1.0-con-ModelSlice>
```

Todo lo demás en este plan es local a `view`.

---

## Diagnóstico (qué viola el arnés hoy)

| # | Síntoma actual | Principio violado | Corrección |
|---|---|---|---|
| 1 | `CanSave()`/`CanDelete()` + error runtime `"Save not allowed"` (`presenter.go:92,109`) | Estado ilegal representable (P3, P6) | Capacidades como interfaces + type assertion en la costura |
| 2 | `project func(list model.FielderSlice) []Item` obliga a cada consumidor a asertar `list.(*XList)`, mantener un `map` cache y coordinarlo con `WithFill` | Hueco no tipado (P1) + pegamento repetido en cada consumidor (P9) | Contrato `Itemizer` por fila; el presenter proyecta e indexa internamente |
| 3 | Check runtime `list.(model.Decodable)` en `Reload` (`presenter.go:53-55`) | Invariante verificada en runtime (P6) | `newList func() model.ModelSlice` → error de compilación |
| 4 | `Select("")` como valor mágico para limpiar selección | Implícito, dos intenciones un camino (P2, P4) | Método explícito `Deselect()` |
| 5 | `SearchPlaceholder()` sin contrato de qué hace la búsqueda | Contrato faltante en la costura (P9) | `Filter(term string) []Item` |
| 6 | `Delete(id)` con id desconocido o `fill == nil` envía `rec = nil` al transporte sin error (`presenter.go:112-118`) | Fallo silencioso (P6) | Error ruidoso; nunca enviar nil |

**Lo que NO se cambia (el arnés lo valida):**

- Los `panic` de `New` ante dependencias nil se **mantienen**: son el
  "loud development diagnostic" del principio 6 (patrón `template.Must`). No se
  reemplazan por `SetLog` ni por `(Presenter, error)`: loguear y continuar produce un
  presenter a medio construir que revienta lejos de la causa — un fallo silencioso
  diferido, el escalón prohibido.
- `Reload`/`Save`/`Delete` síncronos devolviendo `error` se mantienen (Design Goal 4).
- Los ops como `string` se mantienen: `router.Caller.Call(op string, ...)` tipa así la
  costura; un `router.Op` nombrado sería mejora upstream en `router`, fuera de alcance.

---

## API objetivo (contrato completo)

```go
package view

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Item: UNA fila proyectada de la lista. Sin markup.
type Item struct {
	ID          string // clave de selección
	Label       string // texto principal
	Description string // texto secundario
}

// Itemizer: un registro de dominio que sabe proyectarse como fila de lista.
// Es lo ÚNICO específico de view que un módulo implementa en su modelo.
type Itemizer interface {
	Item() Item
}

// Presenter: núcleo de lectura/lista/selección — siempre presente.
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item               // lista proyectada del último Reload
	Filter(term string) []Item   // filtrado local case-insensitive sobre Label+Description; term=="" devuelve todo
	Reload() error               // invoca ListOp, decodifica, proyecta e indexa síncronamente

	Selected() string             // id seleccionado ("" si ninguno)
	Select(id string) model.Model // fija id y devuelve su registro del índice interno; id desconocido → nil, selección intacta
	Deselect()                    // limpia la selección (reemplaza al sentinel Select(""))
}

// Capacidades: el renderer las descubre por type assertion en la costura,
// igual que el composition root descubre router.APIModule.
type Saver interface {
	Save(payload model.Model) error
}
type Deleter interface {
	Delete(id string) error
}

type Option func(*config)

func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option
func WithSaveOp(op string) Option
func WithDeleteOp(op string) Option
func WithArgs(args func() model.Encodable) Option
// WithFill: ELIMINADO — el índice id→Model lo mantiene el presenter.

func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.ModelSlice, // FielderSlice + Decodable, nombrado en model
	opts ...Option,
) Presenter
// El parámetro `project` desaparece: la proyección la aporta cada fila vía Itemizer.
```

---

## Pasos de ejecución (en orden)

### 1. `go.mod` — subir `model`

`go get github.com/tinywasm/model@<versión-con-ModelSlice>` y `go mod tidy`.

### 2. `view.go` — contrato

1. Agregar `Itemizer`.
2. Reescribir `Presenter` según el API objetivo: quitar `CanSave`, `Save`, `CanDelete`,
   `Delete`; agregar `Filter` y `Deselect`.
3. Declarar `Saver` y `Deleter`.
4. `Option` pasa a operar sobre un struct interno `config` (no sobre `*presenter`),
   porque `New` ahora devuelve tipos concretos distintos según capacidades.
5. Eliminar `WithFill`. Mantener `WithTitle`, `WithSearchPlaceholder`, `WithSaveOp`,
   `WithDeleteOp`, `WithArgs`.
6. `New`: quitar los parámetros `project` y el tipo viejo de `newList`; mantener los
   panics existentes para caller/record/listOp/newList nil (diagnóstico ruidoso de
   desarrollo). Al final, envolver según opciones:

```go
c := &core{...}
switch {
case cfg.saveOp != "" && cfg.deleteOp != "":
	return &crud{core: c}        // Presenter + Saver + Deleter
case cfg.saveOp != "":
	return &saveable{core: c}    // Presenter + Saver
case cfg.deleteOp != "":
	return &deletable{core: c}   // Presenter + Deleter
default:
	return c                     // solo Presenter
}
```

La aserción `p.(view.Saver)` es así **veraz**: sin `WithSaveOp`, el método `Save` no
existe en el valor devuelto. El estado ilegal dejó de ser invocable.

### 3. `presenter.go` — implementación

1. Renombrar `presenter` → `core`; agregar structs `saveable`, `deletable`, `crud`
   que embeben `*core` y aportan `Save`/`Delete`.
2. Campo nuevo en `core`: `index map[string]model.Model` (reemplaza al `fill` inyectado).
3. `Reload()`:
   - Ya no necesita el check `list.(model.Decodable)` — `model.ModelSlice` lo garantiza
     en compilación.
   - Tras decodificar, iterar `list.Len()/At(i)` UNA vez aquí (no en cada consumidor):

```go
p.items = p.items[:0]
p.index = make(map[string]model.Model, list.Len())
for i := 0; i < list.Len(); i++ {
	row := list.At(i)
	iz, ok := row.(Itemizer)
	if !ok {
		return fmt.Err("view: Reload: row type does not implement view.Itemizer:", row)
	}
	m, ok := row.(model.Model)
	if !ok {
		return fmt.Err("view: Reload: row type does not implement model.Model:", row)
	}
	it := iz.Item()
	p.items = append(p.items, it)
	p.index[it.ID] = m
}
```

   (Estas dos aserciones son el techo alcanzable en Go — `FielderSlice.At` devuelve
   `Fielder` — pero viven UNA vez dentro de la librería con mensaje ruidoso, no en N
   consumidores.)
4. `Select(id)`: buscar en `p.index`; si existe, fijar `selected` y devolver el modelo;
   si no existe, devolver `nil` sin tocar la selección. Eliminar la rama `id == ""`.
5. `Deselect()`: `p.selected = ""`.
6. `Filter(term)`: substring case-insensitive sobre `Label` y `Description`;
   `term == ""` → copia de `Items()`. Sin llamadas de red (filtrado local; el filtrado
   remoto ya tiene camino: `WithArgs` + `Reload`).
7. `Save(payload)` (solo en `saveable`/`crud`): igual que hoy — nil → error, luego
   `caller.Call(saveOp, payload, nil, done)` sincronizado por canal. Desaparece el check
   `CanSave` (el tipo ya lo garantiza).
8. `Delete(id)` (solo en `deletable`/`crud`): buscar en `p.index`;
   id desconocido → `fmt.Err("view: Delete: unknown id:", id)`. **Nunca** enviar nil.

### 4. `mock/renderer.go` — renderer de referencia

1. `Select(id)`: usar el retorno de `p.Select(id)` como hoy; agregar `Deselect()` que
   llama `p.Deselect()` y limpia el form.
2. `Save()`/`Delete()`: descubrir capacidad por aserción:

```go
func (r *Renderer) Save() error {
	s, ok := r.p.(view.Saver)
	if !ok {
		return fmt.Err("mock: presenter has no Save capability")
	}
	rec := r.p.Record()
	r.syncFormToRecord(rec)
	return s.Save(rec)
}
```

3. Agregar `Filter(term string) []string` (labels filtrados) para ejercitar el contrato.

### 5. `conformance/conformance.go` — cláusulas

Mantener y ajustar:

- `mount_triggers_list_load` — sin cambios.
- `list_renders_item_labels` — la proyección ahora viene de `Itemizer` (el modelo de
  prueba del arnés implementa `Item()`).
- `select_fills_form` — usa el índice interno (ya no hay `WithFill` que inyectar).
- `save_ships_form_values` — obtiene `Save` vía aserción `p.(view.Saver)`.
- `delete_ships_selected_record` — vía `p.(view.Deleter)`.
- `no_save_capability_when_saveop_empty` — pasa de verificar un booleano a verificar
  que la aserción de tipo **falla**: `_, ok := p.(view.Saver); ok == false`.

Cláusulas nuevas:

- `deselect_clears_selection` — `Select(id)` → `Deselect()` → `Selected() == ""`.
- `select_unknown_id_returns_nil` — selección previa queda intacta.
- `filter_matches_label_and_description` — case-insensitive, `""` devuelve todo.
- `delete_unknown_id_errors` — error ruidoso, nada llega al Caller (el fake registra 0
  llamadas).
- `no_delete_capability_when_deleteop_empty` — simétrica a la de Save.

El modelo de prueba interno del arnés debe implementar `view.Itemizer` además de
`model.Model`, y su lista debe satisfacer `model.ModelSlice`.

### 6. `tests/` — actualizar a la nueva firma

`module_test.go` y `conformance_test.go`: quitar `project`/`WithFill`, implementar
`Item()` en el modelo de prueba, y ejecutar `conformance.Run` con la factory nueva.

### 7. README

Ya reescrito como documento oficial de uso (este commit). Verificar al final que cada
firma del README compila tal cual contra el código — el README es el contrato.

### 8. Verificación de cierre

```bash
go build ./... && go vet ./... && go test ./...
```

Regla de cierre del arnés: tras el refactor, los únicos modos de fallo posibles deben
ser **error de compilación** o **diagnóstico ruidoso de desarrollo** — nunca un misterio
en runtime. Revisar cada `return err`/`panic` restante contra esa regla.

---

## Resultado esperado (acid test)

Un agente sin contexto sobre la librería debe poder crear una vista con SOLO esto:

1. Implementar `Item() view.Item` en su registro (3 líneas).
2. Llamar `view.New(caller, &X{}, "x.list", func() model.ModelSlice { return &XList{} },
   view.WithSaveOp("x.save"))`.
3. En el renderer, asertar `p.(view.Saver)` / `p.(view.Deleter)` para saber qué botones
   dibujar.

Si necesita leer más que el README para lograrlo, el arnés sigue abierto.
