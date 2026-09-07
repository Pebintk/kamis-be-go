package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/profile/internal/migrations"
	"github.com/karina/kamis-be-go/services/profile/internal/repository"
)

// newRefreshService builds a UserService against a real database.
//
// The rotation rules are about rows changing state, so they cannot be tested
// without one. It is skipped unless TEST_DATABASE_URL points at a database; CI
// has no Postgres and skips it. To run it:
//
//	podman run -d --name kamis-pg -e POSTGRES_PASSWORD=kamis -e POSTGRES_USER=kamis \
//	  -p 55432:5432 postgres:16-alpine
//	TEST_DATABASE_URL='postgres://kamis:kamis@localhost:55432/postgres?sslmode=disable' \
//	  go test ./services/profile/internal/service/ -run TestRefresh -v
func newRefreshService(t *testing.T) *UserService {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the refresh-token tests")
	}

	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	// Each test starts from an empty table; the rows are the state under test.
	if err := db.Exec("DELETE FROM refresh_tokens").Error; err != nil {
		t.Fatal(err)
	}

	key, pub := testKeyPair(t)
	issuer, err := auth.NewIssuer(key, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_ = pub

	return NewUserService(
		repository.NewUserRepository(db),
		repository.NewRefreshTokenRepository(db),
		issuer,
		7*24*time.Hour,
	)
}

// TestRefreshRotates covers the normal path: a refresh token is single-use, and
// exchanging it yields a new pair.
func TestRefreshRotates(t *testing.T) {
	svc := newRefreshService(t)
	ctx := context.Background()

	first, err := svc.issue(ctx, "tester", "Admin")
	if err != nil {
		t.Fatal(err)
	}

	second, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh returned the same refresh token; it must rotate")
	}
	if second.Token == first.Token {
		t.Error("refresh returned the same access token")
	}
	if second.ExpiresInSecs != int((15 * time.Minute).Seconds()) {
		t.Errorf("expiresInSeconds = %d, want 900", second.ExpiresInSecs)
	}

	// The new token works, which proves rotation did not break the chain.
	if _, err := svc.Refresh(ctx, second.RefreshToken); err != nil {
		t.Errorf("rotated token could not be used: %v", err)
	}
}

// TestRefreshDetectsReplay is the point of rotation. A token presented twice
// means someone kept a copy, so every token the user holds is revoked — the
// victim can log in again, the thief cannot.
func TestRefreshDetectsReplay(t *testing.T) {
	svc := newRefreshService(t)
	ctx := context.Background()

	stolen, err := svc.issue(ctx, "victim", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	live, err := svc.Refresh(ctx, stolen.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	// The thief replays the token the victim already spent.
	if _, err := svc.Refresh(ctx, stolen.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay returned %v, want ErrInvalidRefreshToken", err)
	}

	// The victim's current token is now dead too: one stolen token makes the
	// whole family suspect.
	if _, err := svc.Refresh(ctx, live.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("the victim's live token survived a detected replay: %v", err)
	}
}

// TestLogoutRevokes is what makes logging out real: the refresh token stops
// buying access tokens immediately.
func TestLogoutRevokes(t *testing.T) {
	svc := newRefreshService(t)
	ctx := context.Background()

	session, err := svc.issue(ctx, "tester", "Finance")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, session.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, session.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("a logged-out token still refreshes: %v", err)
	}

	// Logging out twice, or with a token that never existed, is not an error:
	// logout is not a place to confirm what exists.
	if err := svc.Logout(ctx, session.RefreshToken); err != nil {
		t.Errorf("second logout returned %v, want nil", err)
	}
	if err := svc.Logout(ctx, "never-existed"); err != nil {
		t.Errorf("logout of an unknown token returned %v, want nil", err)
	}
}

// TestRefreshRejectsUnknown keeps every failure mode behind one error, so a
// caller cannot learn which of their guesses was once real.
func TestRefreshRejectsUnknown(t *testing.T) {
	svc := newRefreshService(t)

	if _, err := svc.Refresh(context.Background(), "not-a-token"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("unknown token returned %v, want ErrInvalidRefreshToken", err)
	}
}

// TestRefreshKeepsTheRoleFromLogin pins that a refresh does not re-read the
// user, so a role change takes effect on the next login rather than silently
// mid-session.
func TestRefreshKeepsTheRoleFromLogin(t *testing.T) {
	svc := newRefreshService(t)
	ctx := context.Background()

	issued, err := svc.issue(ctx, "tester", "Operasional")
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := svc.Refresh(ctx, issued.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	_, pub := testKeyPair(t)
	verifier, err := auth.NewVerifier(pub)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Parse(refreshed.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Role != "Operasional" {
		t.Errorf("role after refresh = %q, want Operasional", claims.Role)
	}
	if claims.Subject != "tester" {
		t.Errorf("subject after refresh = %q, want tester", claims.Subject)
	}
}
