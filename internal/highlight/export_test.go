package highlight

// HasStyle reports whether a chroma style was resolved, which is otherwise unobservable from outside
// the package.
func (h *Highlighter) HasStyle() bool {
	return h.style != nil
}
