package proton

/* The SessionConfig represents the minim data to Restart _and_ Unlock()
 * a session. This struct is basically the SessionCredentials struct with
 * the addition of the SaltedKeyPass; which is required for the Unock()
 * operation.
 *
 * FIXME Ideally we would be able to store only the SessionCredentials
 * encrypted by the keypass via a SessionAgent. Doing this would simply
 * the code a bit and remove the need for this structure. */
type SessionConfig = struct {
	UID           string `json:"uid"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	SaltedKeyPass string
}
