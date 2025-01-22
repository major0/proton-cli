package proton

/* The SessionCredentials is the minimum data required for restarting a
 * session later. With the exception of the SaltedKeyPass, all of this
 * data is returned by the Client.Login() call, w/ the SaltedKeyPass
 * being a salt of the password+UID.
 *
 * After a succesful login there is a small time window in which the
 * application must call proton.Unlock() to unlock the account.
 * Failure to do so will timeout the authentication process and a new
 * login session will need to be established.
 *
 * WARNING: This information is sensitive and should not be stored in the
 *          clear text anywhere!
 *          See: https://github.com/major0/proton-cli/issues/7 */
type SessionCredentials struct {
	UID          string `json:"uid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
