package drive

// RemoveOpts controls Remove behavior.
type RemoveOpts struct {
	Recursive bool // allow removing non-empty folders
	Permanent bool // permanently delete instead of trash
}
