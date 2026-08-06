package pluginrpc

// ViewRequest asks the plugin for a serializable UI snapshot.
type ViewRequest struct {
	View string // e.g. "keys"; empty = default / current
}

// ConfigureRequest supplies host-resolved connection settings so the plugin
// does not need to call back into the host secrets broker during GetView
// (nested RPC on the same mux session can deadlock net/rpc).
type ConfigureRequest struct {
	Settings map[string]string
}

// KeyBinding describes a host-rendered shortcut that maps to DoAction.
type KeyBinding struct {
	Key    string
	Label  string
	Action string
}

// ViewData is the host-renderable plugin UI snapshot.
type ViewData struct {
	View         string // current view id (e.g. "keys", "info")
	Title        string
	Info         string
	Status       string
	Headers      []string
	Rows         [][]string
	SelectionKey string
	KeyBindings  []KeyBinding
	LogLines     []string
}

// ActionRequest invokes a plugin-side action (refresh, delete, …).
type ActionRequest struct {
	Action  string
	View    string
	Payload map[string]string
}

// ActionResult is returned after DoAction; optional Next replaces cached view.
// ModalTitle/ModalBody ask the host to show an info modal (key content, doctor, etc.).
type ActionResult struct {
	OK         bool
	Message    string
	Next       *ViewData
	ModalTitle string
	ModalBody  string
}
