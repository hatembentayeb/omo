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

// HelpSection is one titled group in the "?" help modal (usually a view).
type HelpSection struct {
	Title    string
	Bindings []KeyBinding
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
	// ViewBindings are digit/view switches shown in the middle header column (0-9).
	ViewBindings []KeyBinding
	// KeyBindings are expanded shortcuts shown in the right header column (former logs).
	KeyBindings []KeyBinding
	// Actions are per-view operations for the host sidebar only (current view).
	Actions []KeyBinding
	// HelpSections drive the "?" modal, grouped by view / topic.
	HelpSections []HelpSection
	LogLines     []string
}

// ActionRequest invokes a plugin-side action (refresh, delete, …).
type ActionRequest struct {
	Action  string
	View    string
	Payload map[string]string
}

// ExternalSession asks the host to suspend the TUI and run a local process
// (e.g. interactive ssh). Password, if set, is fed via SSH_ASKPASS on the host.
type ExternalSession struct {
	Argv     []string // e.g. ["ssh", "-p", "22", "user@host"]
	Password string
	Banner   string
}

// ActionResult is returned after DoAction; optional Next replaces cached view.
// ModalTitle/ModalBody ask the host to show an info modal (key content, doctor, etc.).
type ActionResult struct {
	OK              bool
	Message         string
	Next            *ViewData
	ModalTitle      string
	ModalBody       string
	ExternalSession *ExternalSession
}
