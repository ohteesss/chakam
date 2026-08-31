package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	userapp "chakam/internal/application/user"
	"chakam/internal/domain/user"

	"github.com/gin-gonic/gin"
)

type usersRepository struct {
	users []*user.User
}

func (r *usersRepository) Create(context.Context, *user.User) error { return nil }

func (r *usersRepository) GetByPhone(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (r *usersRepository) GetUserById(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (r *usersRepository) GetAllUsers(_ context.Context, offset, limit int) ([]*user.User, int64, error) {
	if offset >= len(r.users) {
		return []*user.User{}, int64(len(r.users)), nil
	}
	end := offset + limit
	if end > len(r.users) {
		end = len(r.users)
	}
	return r.users[offset:end], int64(len(r.users)), nil
}

func TestGetUsersReturnsPaginatedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := []*user.User{
		{PublicID: "1"}, {PublicID: "2"}, {PublicID: "3"}, {PublicID: "4"}, {PublicID: "5"},
	}
	api := NewApiHandler(nil, userapp.NewService(&usersRepository{users: users}))
	engine := gin.New()
	engine.GET("/users", api.GetUsers)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users?page=2&page_size=2", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
		Pagination struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			Total      int64 `json:"total"`
			TotalPages int64 `json:"total_pages"`
			HasNext    bool  `json:"has_next"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Users) != 2 || response.Users[0].ID != "3" || response.Users[1].ID != "4" {
		t.Fatalf("users = %#v, want IDs 3 and 4", response.Users)
	}
	if response.Pagination.Page != 2 || response.Pagination.PageSize != 2 || response.Pagination.Total != 5 || response.Pagination.TotalPages != 3 || !response.Pagination.HasNext {
		t.Fatalf("pagination = %#v, want page 2, size 2, total 5, total_pages 3, has_next true", response.Pagination)
	}
}

func TestGetUsersRejectsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := NewApiHandler(nil, userapp.NewService(&usersRepository{}))
	engine := gin.New()
	engine.GET("/users", api.GetUsers)

	for _, path := range []string{"/users?page=0", "/users?page_size=101", "/users?page=abc"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusBadRequest)
		}
	}
}
