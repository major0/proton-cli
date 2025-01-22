package internal

import (
	"encoding/json"
	"log/slog"

	"github.com/adrg/xdg"
	"github.com/major0/proton-cli/proton"
	tkv "github.com/miteshbsjat/textfilekv"
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
type FileStore struct {
	accountName string
	fileName    string
}

func NewFileStore(filename string, account string) *FileStore {
	slog.Warn("Use of a file-backed session store is not secure. See: https://github.com/major0/proton-cli/issues/7")

	return &FileStore{
		fileName:    filename,
		accountName: account,
	}
}

func (fs *FileStore) Load() (*proton.SessionConfig, error) {
	sessionCachePath, err := xdg.CacheFile(fs.fileName)
	if err != nil {
		return nil, err
	}
	if sessionCachePath == "" {
		return nil, nil
	}

	slog.Debug("FileStore.Load()", "sessionCachePath", sessionCachePath)

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return nil, err
	}

	val, ok := kvs.Get(fs.accountName)
	if !ok {
		return nil, proton.ErrKeyNotFound
	}

	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}

	config := proton.SessionConfig{}
	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (fs *FileStore) Save(session *proton.SessionConfig) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	sessionCachePath, err := xdg.CacheFile(fs.fileName)
	if err != nil {
		return err
	}

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return err
	}

	err = kvs.Set(fs.accountName, string(data))
	if err != nil {
		return err
	}

	return nil
}

func (fs *FileStore) Delete() error {
	sessionCachePath, err := xdg.CacheFile(fs.fileName)
	if err != nil {
		return err
	}

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return err
	}

	return kvs.Delete(fs.accountName)
}

func (fs *FileStore) List() ([]string, error) {
	sessionCachePath, err := xdg.CacheFile(fs.fileName)
	if err != nil {
		return nil, err
	}

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return nil, err
	}

	return kvs.Keys(), nil
}

func (fs *FileStore) Switch(account string) error {
	// FIXME validate that the account exists in the store
	fs.accountName = account
	return nil
}
