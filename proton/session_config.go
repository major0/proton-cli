package proton

type SessionConfig = struct {
	UID           string `json:"uid"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	SaltedKeyPass string
}
