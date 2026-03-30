package proton

// Block is a block of file contents. They are split in 4MB blocks although this number may change in the future.
// Each block is its own data packet separated from the key packet which is held by the node,
// which means the sessionKey is the same for every block.
type Block struct {
	Index int

	BareURL string // URL to the block
	Token   string // Token for download URL

	Hash           string // Encrypted block's sha256 hash, in base64
	EncSignature   string // Encrypted signature of the block
	SignatureEmail string // Email used to sign the block
}

type BlockUploadReq struct {
	AddressID  string
	ShareID    string
	LinkID     string
	RevisionID string

	BlockList []BlockUploadInfo
}

type BlockUploadInfo struct {
	Index        int
	Size         int64
	EncSignature string
	Hash         string
	Verifier     BlockUploadVerifier `json:",omitempty"`
}

// BlockUploadVerifier holds the per-block verification token required by the
// Proton storage backend to authenticate that the block was correctly encrypted.
type BlockUploadVerifier struct {
	Token string `json:",omitempty"`
}

// RevisionVerification is the response from the block upload verification endpoint.
type RevisionVerification struct {
	VerificationCode string // Base64-encoded verification code XOR'd with each block
	ContentKeyPacket string // Encrypted content session key (for client-side integrity check)
}

type BlockUploadLink struct {
	Token   string
	BareURL string
}
