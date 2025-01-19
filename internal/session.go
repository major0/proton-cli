package internal

import (
	"encoding/json"
	"log/slog"

	"github.com/major0/proton-cli/proton"
)

/*
The SessionStore is used to cache the session information. This allows
* a session to be restarted inbetween commands without having to login
* with a username and password.
*
* FIXME While using a local file store is fine during development, it is
* not a very viable long-term solution for production use. Various
* solutions exist for securely storing secrets that will handle automatic
* interaction with the user. OSX even supports such a solution for the
* entire platform.

* In the long run we should see about supporting the default local secrets
* store for whatever platform we're running on, while also allowing the user
* to configure their own secrets agent.
*
* Ref: https://github.com/keybase/go-keychain
*/
type SessionStore struct {
	FileCache   *FileCache
	AccountName string
}

func NewSessionStore(cache *FileCache, account string) *SessionStore {
	slog.Warn("Use of a file-backed session store is not secure. See: https://github.com/major0/proton-cli/issues/7")
	return &SessionStore{
		FileCache:   cache,
		AccountName: account,
	}
}

func (ss *SessionStore) Load() (*proton.SessionConfig, error) {
	data, err := ss.FileCache.Get(ss.AccountName)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	session := proton.SessionConfig{}
	err = json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (ss *SessionStore) Save(session *proton.SessionConfig) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	err = ss.FileCache.Put(ss.AccountName, data)
	if err != nil {
		return err
	}

	return nil
}

func (ss *SessionStore) Delete() error {
	return ss.FileCache.Delete(ss.AccountName)
}
