package user

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domain "chakam/internal/domain/user"
)

// fakeRepo is an in-memory Repository. This is the whole payoff of the port:
// the service can be tested with zero database, because it only ever sees the
// interface. A real test of the GORM repo would be separate (and DB-backed).
type fakeRepo struct {
	byPhone map[string]*domain.User
	users   []*domain.User
	creates int
	offset  int
	limit   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byPhone: map[string]*domain.User{}}
}

func (f *fakeRepo) GetByPhone(_ context.Context, phone string) (*domain.User, error) {
	if u, ok := f.byPhone[phone]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, u *domain.User) error {
	f.byPhone[u.PhoneNumber] = u
	f.users = append(f.users, u)
	f.creates++
	return nil
}

// GetUserById added because the Repository interface grew. The fake must
// implement EVERY method or it no longer satisfies the interface (that's the
// compile error you saw). We look up by PublicID to mirror the real query.
func (f *fakeRepo) GetUserById(_ context.Context, userId string) (*domain.User, error) {
	for _, u := range f.byPhone {
		if u.PublicID == userId {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) GetAllUsers(_ context.Context, offset, limit int) ([]*domain.User, int64, error) {
	f.offset = offset
	f.limit = limit
	if offset >= len(f.users) {
		return []*domain.User{}, int64(len(f.users)), nil
	}
	end := min(offset+limit, len(f.users))
	return f.users[offset:end], int64(len(f.users)), nil
}

func TestCreateUser_NewUser(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	u, err := svc.CreateUser(context.Background(), "+2348031234567", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Assert it parses as a real UUID — stronger than a length check.
	if _, err := uuid.Parse(u.PublicID); err != nil {
		t.Errorf("PublicID %q is not a valid UUID: %v", u.PublicID, err)
	}
	if u.PhoneNumber != "+2348031234567" {
		t.Errorf("phone = %q, want %q", u.PhoneNumber, "+2348031234567")
	}
	if repo.creates != 1 {
		t.Errorf("repo.Create called %d times, want 1", repo.creates)
	}
}

func TestCreateUser_Idempotent(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	first, _ := svc.CreateUser(context.Background(), "+2348031234567", nil)
	second, err := svc.CreateUser(context.Background(), "+2348031234567", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second call must NOT create a duplicate — it returns the existing user.
	if repo.creates != 1 {
		t.Errorf("repo.Create called %d times, want 1 (idempotent)", repo.creates)
	}
	if first.PublicID != second.PublicID {
		t.Errorf("expected the same user back, got different PublicIDs")
	}
}

func TestGetAllUsers_Paginates(t *testing.T) {
	repo := newFakeRepo()
	for i := range 5 {
		repo.users = append(repo.users, &domain.User{PublicID: string(rune('a' + i))})
	}
	svc := NewService(repo)

	result, err := svc.GetAllUsers(context.Background(), 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.offset != 2 || repo.limit != 2 {
		t.Fatalf("repository called with offset=%d, limit=%d; want 2, 2", repo.offset, repo.limit)
	}
	if result.Total != 5 || len(result.Users) != 2 {
		t.Fatalf("result = total %d with %d users; want total 5 with 2 users", result.Total, len(result.Users))
	}
	if result.Users[0].PublicID != "c" || result.Users[1].PublicID != "d" {
		t.Fatalf("users = %q, %q; want c, d", result.Users[0].PublicID, result.Users[1].PublicID)
	}
}

func TestGetAllUsers_RejectsInvalidPagination(t *testing.T) {
	svc := NewService(newFakeRepo())

	if _, err := svc.GetAllUsers(context.Background(), 0, DefaultPageSize); !errors.Is(err, ErrInvalidPagination) {
		t.Fatalf("page 0 error = %v, want ErrInvalidPagination", err)
	}
	if _, err := svc.GetAllUsers(context.Background(), 1, MaxPageSize+1); !errors.Is(err, ErrInvalidPagination) {
		t.Fatalf("oversized page_size error = %v, want ErrInvalidPagination", err)
	}
}

// The users table previously held one person four times, because each entry
// point stored the number in whatever format it arrived in: a wa_id from the
// webhook, a local number typed into chat, and an E.164 value with a stray
// space. CreateUser normalises first, so every spelling now resolves to the
// same record.
func TestCreateUser_CollapsesEquivalentFormats(t *testing.T) {
	svc := NewService(newFakeRepo())

	first, err := svc.CreateUser(context.Background(), "+2348031234567", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spellings := []string{
		"+2348031234567",    // already canonical
		"+234 803 123 4567", // E.164 with spaces
		"+234-803-123-4567", // E.164 with dashes
	}
	for _, s := range spellings {
		u, err := svc.CreateUser(context.Background(), s, nil)
		if err != nil {
			t.Fatalf("CreateUser(%q) returned error: %v", s, err)
		}
		if u.PublicID != first.PublicID {
			t.Errorf("CreateUser(%q) created a second identity: got %s, want %s", s, u.PublicID, first.PublicID)
		}
		if u.PhoneNumber != "+2348031234567" {
			t.Errorf("CreateUser(%q) stored %q, want canonical E.164", s, u.PhoneNumber)
		}
	}
}

// A number that has not been normalised must be refused rather than silently
// stored, so a new entry point cannot reintroduce duplicate identities.
func TestCreateUser_RejectsNonCanonical(t *testing.T) {
	svc := NewService(newFakeRepo())

	for _, bad := range []string{
		"08031234567",   // local form: ambiguous without a region
		"2348031234567", // wa_id: missing the leading "+"
		"not-a-phone",
		"",
	} {
		if _, err := svc.CreateUser(context.Background(), bad, nil); err == nil {
			t.Errorf("CreateUser(%q) succeeded, want an error", bad)
		}
	}
}
