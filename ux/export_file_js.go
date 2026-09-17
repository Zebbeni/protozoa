package ux

import "syscall/js"

// saveExport downloads data as a file called name through the browser
// and returns the name, since there's no filesystem to write to.
func saveExport(name string, data []byte) (string, error) {
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	blob := js.Global().Get("Blob").New([]any{bytes}, map[string]any{"type": "application/json"})
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	link := js.Global().Get("document").Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", name)
	link.Call("click")
	js.Global().Get("URL").Call("revokeObjectURL", url)
	return name + " (downloaded)", nil
}
