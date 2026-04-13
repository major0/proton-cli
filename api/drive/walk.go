package drive

// WalkOrder controls tree traversal order.
type WalkOrder int

const (
	// BreadthFirst emits all entries at depth N before any at depth N+1.
	BreadthFirst WalkOrder = iota
	// DepthFirst emits directory contents before the directory itself.
	DepthFirst
)

// WalkEntry is a single entry yielded by TreeWalk.
type WalkEntry struct {
	Path      string // constructed traversal path (ephemeral, from EntryName values)
	Link      *Link  // raw encrypted link — consumer triggers lazy decryption
	Depth     int    // depth from walk root (root = 0, root children = 1)
	EntryName string // propagated from NamedDirEntry.EntryName
}
