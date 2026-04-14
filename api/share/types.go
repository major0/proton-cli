// Package share defines types for Proton Drive share membership and
// invitation management. This is the types layer — it does not make
// API calls. HTTP operations live in api/share/client/.
package share

// Permission bitmask values for share members.
const (
	PermRead  = 4
	PermWrite = 2
	PermAdmin = 16

	PermViewer = PermRead              // 4
	PermEditor = PermRead | PermWrite  // 6
)

// FormatPermissions returns a human-readable label for a permission bitmask.
func FormatPermissions(p int) string {
	switch {
	case p&PermAdmin != 0:
		return "admin"
	case p == PermEditor:
		return "editor"
	case p == PermViewer:
		return "viewer"
	default:
		return "unknown"
	}
}

// Member represents an existing member of a share.
type Member struct {
	MemberID            string `json:"MemberID"`
	Email               string `json:"Email"`
	InviterEmail        string `json:"InviterEmail"`
	AddressID           string `json:"AddressID"`
	CreateTime          int64  `json:"CreateTime"`
	ModifyTime          int64  `json:"ModifyTime"`
	Permissions         int    `json:"Permissions"`
	KeyPacketSignature  string `json:"KeyPacketSignature"`
	SessionKeySignature string `json:"SessionKeySignature"`
}

// Invitation represents a pending invite for a Proton user.
type Invitation struct {
	InvitationID       string `json:"InvitationID"`
	InviterEmail       string `json:"InviterEmail"`
	InviteeEmail       string `json:"InviteeEmail"`
	Permissions        int    `json:"Permissions"`
	KeyPacket          string `json:"KeyPacket"`
	KeyPacketSignature string `json:"KeyPacketSignature"`
	CreateTime         int64  `json:"CreateTime"`
	State              int    `json:"State"`
}

// ExternalInvitation represents a pending invite for a non-Proton email.
type ExternalInvitation struct {
	ExternalInvitationID        string `json:"ExternalInvitationID"`
	InviterEmail                string `json:"InviterEmail"`
	InviteeEmail                string `json:"InviteeEmail"`
	CreateTime                  int64  `json:"CreateTime"`
	Permissions                 int    `json:"Permissions"`
	State                       int    `json:"State"`
	ExternalInvitationSignature string `json:"ExternalInvitationSignature"`
}

// InviteProtonUserPayload is the request body for creating a Proton-user invitation.
type InviteProtonUserPayload struct {
	Invitation struct {
		InviterEmail         string `json:"InviterEmail"`
		InviteeEmail         string `json:"InviteeEmail"`
		Permissions          int    `json:"Permissions"`
		KeyPacket            string `json:"KeyPacket"`
		KeyPacketSignature   string `json:"KeyPacketSignature"`
		ExternalInvitationID string `json:"ExternalInvitationID,omitempty"`
	} `json:"Invitation"`
}

// InviteExternalUserPayload is the request body for creating an external-user invitation.
type InviteExternalUserPayload struct {
	ExternalInvitation struct {
		InviterAddressID            string `json:"InviterAddressID"`
		InviteeEmail                string `json:"InviteeEmail"`
		Permissions                 int    `json:"Permissions"`
		ExternalInvitationSignature string `json:"ExternalInvitationSignature"`
	} `json:"ExternalInvitation"`
}

// Response wrappers used by the client layer to unmarshal API responses.

// MembersResponse wraps the list-members API response.
type MembersResponse struct {
	Code    int      `json:"Code"`
	Members []Member `json:"Members"`
}

// InvitationsResponse wraps the list-invitations API response.
type InvitationsResponse struct {
	Code        int          `json:"Code"`
	Invitations []Invitation `json:"Invitations"`
}

// ExternalInvitationsResponse wraps the list-external-invitations API response.
type ExternalInvitationsResponse struct {
	Code                int                  `json:"Code"`
	ExternalInvitations []ExternalInvitation `json:"ExternalInvitations"`
}

// CreateDriveSharePayload is the request body for POST /drive/volumes/{volumeID}/shares.
// Matches the WebClients CreateDriveShare interface.
type CreateDriveSharePayload struct {
	AddressID                string `json:"AddressID"`
	RootLinkID               string `json:"RootLinkID"`
	ShareKey                 string `json:"ShareKey"`
	SharePassphrase          string `json:"SharePassphrase"`
	SharePassphraseSignature string `json:"SharePassphraseSignature"`
	PassphraseKeyPacket      string `json:"PassphraseKeyPacket"`
	NameKeyPacket            string `json:"NameKeyPacket"`
}

// CreateShareResponse wraps the create-share API response.
type CreateShareResponse struct {
	Code  int `json:"Code"`
	Share struct {
		ID string `json:"ID"`
	} `json:"Share"`
}
