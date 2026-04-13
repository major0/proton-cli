package drive

// WalkOrder controls tree traversal order.
type WalkOrder int

const (
	// BreadthFirst emits all entries at depth N before any at depth N+1.
	BreadthFirst WalkOrder = iota
	// DepthFirst emits directory contents before the directory itself.
	DepthFirst
)
