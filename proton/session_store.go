package proton

type SessionStore interface {
	Load() (*SessionConfig, error)
	Save(session *SessionConfig) error
	Delete() error
	List() ([]string, error)
	Switch(account string) error
}
