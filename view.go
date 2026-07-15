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
