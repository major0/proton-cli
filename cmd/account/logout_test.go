package accountCmd

import (
	"fmt"
	"strings"
	"testing"

	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
)

// trackingStore is a SessionStore that tracks Delete calls and can return errors.
type trackingStore struct {
	failingStore
	deleted   bool
	deleteErr error
}

func (s *trackingStore) Delete() error {
	s.deleted = true
	return s.deleteErr
}

// TestLogout_DeletesCookieAndAccountStore verifies that logout deletes both
// the session store and the cookie store.
func TestLogout_DeletesCookieAndAccountStore(t *testing.T) {
	origStore := cli.SessionStoreVar
	origForce := authLogoutForce
	origCookieDelete := logoutCookieDeleteFn
	t.Cleanup(func() {
		cli.SessionStoreVar = origStore
		authLogoutForce = origForce
		logoutCookieDeleteFn = origCookieDelete
	})

	sessionStore := &trackingStore{failingStore: failingStore{err: common.ErrKeyNotFound}}
	cli.SessionStoreVar = sessionStore
	authLogoutForce = false

	var cookieDeleted bool
	logoutCookieDeleteFn = func() error {
		cookieDeleted = true
		return nil
	}

	err := authLogoutCmd.RunE(authLogoutCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sessionStore.deleted {
		t.Error("session store Delete was not called")
	}
	if !cookieDeleted {
		t.Error("cookie store Delete was not called")
	}
}

// TestLogout_ForceLogoutContinuesOnRestoreFailure verifies that with --force,
// logout continues even when session restore fails.
func TestLogout_ForceLogoutContinuesOnRestoreFailure(t *testing.T) {
	origStore := cli.SessionStoreVar
	origForce := authLogoutForce
	origCookieDelete := logoutCookieDeleteFn
	t.Cleanup(func() {
		cli.SessionStoreVar = origStore
		authLogoutForce = origForce
		logoutCookieDeleteFn = origCookieDelete
	})

	cli.SessionStoreVar = &trackingStore{
		failingStore: failingStore{err: fmt.Errorf("disk error")},
	}
	authLogoutForce = true

	var cookieDeleted bool
	logoutCookieDeleteFn = func() error {
		cookieDeleted = true
		return nil
	}

	err := authLogoutCmd.RunE(authLogoutCmd, nil)
	if err != nil {
		t.Fatalf("force logout should not fail, got: %v", err)
	}
	if !cookieDeleted {
		t.Error("cookie store Delete was not called during force logout")
	}
}

// TestLogout_CookieStoreDeleteFailureLogged verifies that a cookie store
// delete failure is logged but does not fail the logout.
func TestLogout_CookieStoreDeleteFailureLogged(t *testing.T) {
	origStore := cli.SessionStoreVar
	origForce := authLogoutForce
	origCookieDelete := logoutCookieDeleteFn
	t.Cleanup(func() {
		cli.SessionStoreVar = origStore
		authLogoutForce = origForce
		logoutCookieDeleteFn = origCookieDelete
	})

	sessionStore := &trackingStore{failingStore: failingStore{err: common.ErrKeyNotFound}}
	cli.SessionStoreVar = sessionStore
	authLogoutForce = false

	logoutCookieDeleteFn = func() error {
		return fmt.Errorf("cookie keyring locked")
	}

	// Logout should succeed even though cookie delete fails.
	err := authLogoutCmd.RunE(authLogoutCmd, nil)
	if err != nil {
		t.Fatalf("logout should succeed despite cookie delete failure, got: %v", err)
	}
	if !sessionStore.deleted {
		t.Error("session store Delete was not called")
	}
}

// TestLogout_RestoreErrorWithoutForce verifies that a non-ErrNotLoggedIn
// restore error is returned when --force is not set.
func TestLogout_RestoreErrorWithoutForce(t *testing.T) {
	origStore := cli.SessionStoreVar
	origForce := authLogoutForce
	origCookieDelete := logoutCookieDeleteFn
	t.Cleanup(func() {
		cli.SessionStoreVar = origStore
		authLogoutForce = origForce
		logoutCookieDeleteFn = origCookieDelete
	})

	cli.SessionStoreVar = &failingStore{err: fmt.Errorf("disk error")}
	authLogoutForce = false

	logoutCookieDeleteFn = func() error { return nil }

	err := authLogoutCmd.RunE(authLogoutCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "disk error") {
		t.Errorf("error = %q, want substring %q", err.Error(), "disk error")
	}
}
