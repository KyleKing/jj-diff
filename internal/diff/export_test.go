package diff

// Unexported helpers the black-box test package exercises directly.
var (
	ExpandWithContext     = expandWithContext
	RecalculateHunkHeader = recalculateHunkHeader
)
