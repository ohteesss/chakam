package handler

import (
	"errors"
	"net/http"
	"strconv"

	"chakam/internal/application/agreement"
	"chakam/internal/application/user"
	domainagreement "chakam/internal/domain/agreement"
	"chakam/internal/pkg/validator"

	"github.com/gin-gonic/gin"
)

type ApiHandler struct {
	agreementService *agreement.Service
	userService      *user.Service
}

func NewApiHandler(agreementService *agreement.Service, userService *user.Service) *ApiHandler {
	return &ApiHandler{agreementService: agreementService, userService: userService}
}

func (h *ApiHandler) GetAgreementById(c *gin.Context) {
	a, err := h.agreementService.GetAgreementById(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, domainagreement.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agreement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve agreement"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agreement": a})
}

func (h *ApiHandler) GetAgreements(c *gin.Context) {
	var userID string
	phone := c.Query("phone")
	filter := c.Query("filter")
	if filter == "" {
		filter = "all"
	}
	validFilters := map[string]bool{"all": true, "pending": true, "confirmed": true, "created_by_me": true}
	if !validFilters[filter] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filter"})
		return
	}
	if user_id := c.Query("user_id"); user_id != "" {
		userID = user_id
	} else {
		if phone == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or phone is required"})
			return
		}
		// This is an internal API with no sender context to infer a region from,
		// so the caller must supply a full international number. FromWaID is used
		// rather than ToE164 because a "+" in a query string decodes to a space:
		// "?phone=+2348031234567" reaches us as " 2348031234567". Accepting the
		// number with or without its "+" handles that without becoming ambiguous,
		// since a local form like "08031234567" still fails the country-code check.
		phoneE164, err := validator.FromWaID(phone)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "phone must be an international number, e.g. %2B2348031234567 or 2348031234567"})
			return
		}
		u, err := h.userService.CreateUser(c.Request.Context(), phoneE164, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve user"})
			return
		}
		userID = u.PublicID
	}

	agreements, err := h.agreementService.GetAgreements(c.Request.Context(), filter, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve agreements"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agreements": agreements})
}

func (h *ApiHandler) GetUsers(c *gin.Context) {
	page, err := parsePositiveQueryInt(c, "page", 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
		return
	}
	pageSize, err := parsePositiveQueryInt(c, "page_size", user.DefaultPageSize)
	if err != nil || pageSize > user.MaxPageSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_size must be between 1 and 100"})
		return
	}
	users, err := h.userService.GetAllUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		if errors.Is(err, user.ErrInvalidPagination) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page is too large"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve users"})
		return
	}

	totalPages := users.Total / int64(users.PageSize)
	if users.Total%int64(users.PageSize) != 0 {
		totalPages++
	}
	pageOffset := (users.Page - 1) * users.PageSize
	c.JSON(http.StatusOK, gin.H{
		"users": users.Users,
		"pagination": gin.H{
			"page":        users.Page,
			"page_size":   users.PageSize,
			"total":       users.Total,
			"total_pages": totalPages,
			"has_next":    users.Total-int64(pageOffset) > int64(users.PageSize),
		},
	})
}

func parsePositiveQueryInt(c *gin.Context, key string, defaultValue int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("must be a positive integer")
	}
	return parsed, nil
}
