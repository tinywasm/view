package view

import (
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

// Presenter es el motor agnóstico de tecnología detrás de cualquier vista CRUD: listar,
// seleccionar, guardar, eliminar — manejado SOLO por el Caller y el model.Model de forma síncrona.
// Sin DOM, sin form, y sin callbacks colgados.
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item                 // la lista decodificada actual (resultado del último Reload)
	Reload() error                 // invoca ListOp y decodifica en Items síncronamente; retorna error

	Selected() string              // el id que Select fijó por última vez ("" si ninguno)
	Select(id string) model.Model  // marca id como actual y devuelve su registro (via Fill); id=="" limpia y devuelve nil

	CanSave() bool                 // ¿se ofreció SaveOp?
	Save(payload model.Model) error // envía el payload explícito a SaveOp síncronamente

	CanDelete() bool               // ¿se ofreció DeleteOp?
	Delete(id string) error        // envía el registro completo de id (via Fill) a DeleteOp síncronamente
}

// Option representa una opción funcional de configuración opcional para el presentador.
type Option func(*presenter)

// WithTitle asigna un título a la vista.
func WithTitle(title string) Option {
	return func(p *presenter) {
		p.title = title
	}
}

// WithSearchPlaceholder asigna un texto de ayuda para la barra de búsqueda.
func WithSearchPlaceholder(placeholder string) Option {
	return func(p *presenter) {
		p.searchPlaceholder = placeholder
	}
}

// WithSaveOp asigna la operación de guardado (SaveOp).
func WithSaveOp(op string) Option {
	return func(p *presenter) {
		p.saveOp = op
	}
}

// WithDeleteOp asigna la operación de eliminación (DeleteOp).
func WithDeleteOp(op string) Option {
	return func(p *presenter) {
		p.deleteOp = op
	}
}

// WithArgs inyecta una función para obtener los argumentos de la ListOp.
func WithArgs(args func() model.Encodable) Option {
	return func(p *presenter) {
		p.args = args
	}
}

// WithFill inyecta una función para mapear un id a su modelo completo (típicamente de la caché).
func WithFill(fill func(id string) model.Model) Option {
	return func(p *presenter) {
		p.fill = fill
	}
}

// New construye el Presenter a partir de sus componentes requeridos invariantes
// en tiempo de compilación, más opciones funcionales adicionales.
func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.FielderSlice,
	project func(list model.FielderSlice) []Item,
	opts ...Option,
) Presenter {
	if caller == nil {
		panic("view: New: caller is required")
	}
	if model.IsNil(record) {
		panic("view: New: record is required")
	}
	if listOp == "" {
		panic("view: New: listOp is required")
	}
	if newList == nil {
		panic("view: New: newList is required")
	}
	if project == nil {
		panic("view: New: project is required")
	}

	p := &presenter{
		caller:  caller,
		record:  record,
		listOp:  listOp,
		newList: newList,
		project: project,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}
