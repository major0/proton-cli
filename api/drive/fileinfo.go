package drive

// FileInfo provides POSIX stat()-like metadata for a Link.
// Name is lazy — it calls Link.Name() which decrypts on demand.
type FileInfo struct {
	LinkID     string
	Name       func() (string, error)
	Size       int64
	ModifyTime int64
	CreateTime int64
	MIMEType   string
	IsDir      bool
	BlockSizes []int64
}
