package drive

import "github.com/ProtonMail/go-proton-api"

// Volume represents a Proton Drive volume.
// API-calling methods (ListShareMetadata, GetShareMetadata, GetShare)
// live on api/drive/client.Client.
type Volume struct {
	ProtonVolume proton.Volume
}
