package internal

import (
	"errors"
	"log/slog"

	"github.com/adrg/xdg"
	tkv "github.com/miteshbsjat/textfilekv"
)

/* The FileCache is used by the Session type to load/store per-account
 * session information. One of the goals of the store is to support
 * simultaneous access between concurrent executions of the program.
 * To this end, the store must support _either_ a file-level exclusive
 * lock during reading/writing, or a per-record lock during updating.
 * Since this is not a performance-critical operation, it is fine to
 * simply open/close the database during every operation. */
type FileCache struct {
	Filename string
}

var (
	ErrKeyNotFound = errors.New("key not found")
)

func NewFileCache(filename string) *FileCache {
	return &FileCache{
		Filename: filename,
	}
}

func (sc *FileCache) Get(key string) ([]byte, error) {
	sessionCachePath, err := xdg.CacheFile(sc.Filename)
	if err != nil {
		return nil, err
	}
	if sessionCachePath == "" {
		return nil, nil
	}

	slog.Debug("FileCache.Get()", "sessionCachePath", sessionCachePath)

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return nil, err
	}

	val, ok := kvs.Get(key)
	if !ok {
		return nil, ErrKeyNotFound
	}

	return []byte(val), nil
}

func (sc *FileCache) Put(key string, val []byte) error {
	sessionCachePath, err := xdg.CacheFile(sc.Filename)
	if err != nil {
		return err
	}

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return err
	}

	err = kvs.Set(key, string(val))
	if err != nil {
		return err
	}

	return nil
}

func (sc *FileCache) Delete(key string) error {
	sessionCachePath, err := xdg.CacheFile(sc.Filename)
	if err != nil {
		return err
	}

	kvs, err := tkv.NewKeyValueStore(sessionCachePath)
	if err != nil {
		return err
	}

	return kvs.Delete(key)
}
