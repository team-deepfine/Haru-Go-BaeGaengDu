package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/router"
	"github.com/daewon/haru/internal/service"
	jwtpkg "github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/oauth"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type testServer struct {
	engine     *gin.Engine
	db         *gorm.DB
	jwtManager *jwtpkg.Manager
	userRepo   repository.UserRepository
	tokenRepo  repository.TokenRepository
	eventRepo  repository.EventRepository
}

type testUser struct {
	ID           uuid.UUID
	AccessToken  string
	RefreshToken string
}

// noopVoiceParsingService mirrors the unexported one in cmd/server/main.go.
type noopVoiceParsingService struct{}

func (s *noopVoiceParsingService) ParseVoice(_ context.Context, _ service.ParseVoiceInput) (*dto.ParseVoiceResponse, error) {
	return nil, model.ErrAIServiceUnavailable
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:test_%s?mode=memory&cache=shared", uuid.New().String()[:8])
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&model.User{}, &model.RefreshToken{}, &model.Event{})
	require.NoError(t, err)

	jwtManager := jwtpkg.NewManager("integration-test-secret", time.Hour, 720*time.Hour)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	eventRepo := repository.NewEventRepository(db)

	appleVerifier := oauth.NewAppleVerifier("test-client-id")
	authSvc := service.NewAuthService(userRepo, tokenRepo, jwtManager, appleVerifier)
	authHandler := handler.NewAuthHandler(authSvc)

	eventSvc := service.NewEventService(eventRepo)
	eventHandler := handler.NewEventHandler(eventSvc)

	voiceHandler := handler.NewVoiceHandler(&noopVoiceParsingService{})

	engine := router.New(jwtManager, authHandler, eventHandler, voiceHandler)

	return &testServer{
		engine:     engine,
		db:         db,
		jwtManager: jwtManager,
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		eventRepo:  eventRepo,
	}
}

func createTestUser(t *testing.T, ts *testServer) testUser {
	t.Helper()
	ctx := context.Background()
	userID := uuid.Must(uuid.NewV7())
	now := time.Now()
	email := "test-" + userID.String() + "@example.com"

	user := &model.User{
		ID:          userID,
		Provider:    "apple",
		ProviderSub: "sub-" + userID.String(),
		Email:       &email,
		LastLoginAt: &now,
	}
	err := ts.userRepo.Create(ctx, user)
	require.NoError(t, err)

	tokenPair, err := ts.jwtManager.GenerateTokenPair(userID)
	require.NoError(t, err)

	rt := &model.RefreshToken{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		Token:     tokenPair.RefreshToken,
		ExpiresAt: now.Add(720 * time.Hour),
	}
	err = ts.tokenRepo.Create(ctx, rt)
	require.NoError(t, err)

	return testUser{
		ID:           userID,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}
}

func doRequest(ts *testServer, method, path string, body any, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	ts.engine.ServeHTTP(w, req)
	return w
}

func parseJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err, "failed to parse response body: %s", w.Body.String())
	return result
}

// ---------------------------------------------------------------------------
// 1. Health Check
// ---------------------------------------------------------------------------

func TestIntegration_HealthCheck(t *testing.T) {
	ts := setupTestServer(t)

	w := doRequest(ts, http.MethodGet, "/health", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
}

// ---------------------------------------------------------------------------
// 2. Auth Middleware
// ---------------------------------------------------------------------------

func TestIntegration_AuthMiddleware(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("rejects missing auth header", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/auth/me", nil, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/auth/me", nil, "invalid-token-string")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects token signed with wrong secret", func(t *testing.T) {
		wrongManager := jwtpkg.NewManager("wrong-secret-key", time.Hour, 720*time.Hour)
		pair, err := wrongManager.GenerateTokenPair(uuid.New())
		require.NoError(t, err)

		w := doRequest(ts, http.MethodGet, "/api/auth/me", nil, pair.AccessToken)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// ---------------------------------------------------------------------------
// 3. Auth Flow
// ---------------------------------------------------------------------------

func TestIntegration_AuthFlow(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	t.Run("GET /api/auth/me returns current user", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/auth/me", nil, user.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.UserResponse](t, w)
		assert.Equal(t, user.ID.String(), resp.ID)
		assert.Equal(t, "apple", resp.Provider)
	})

	t.Run("POST /api/auth/refresh rotates tokens", func(t *testing.T) {
		refreshUser := createTestUser(t, ts)

		body := dto.RefreshRequest{RefreshToken: refreshUser.RefreshToken}
		w := doRequest(ts, http.MethodPost, "/api/auth/refresh", body, "")
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.AuthResponse](t, w)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.NotEqual(t, refreshUser.RefreshToken, resp.RefreshToken)

		// New access token works
		w2 := doRequest(ts, http.MethodGet, "/api/auth/me", nil, resp.AccessToken)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Old refresh token no longer works (rotation)
		w3 := doRequest(ts, http.MethodPost, "/api/auth/refresh",
			dto.RefreshRequest{RefreshToken: refreshUser.RefreshToken}, "")
		assert.Equal(t, http.StatusUnauthorized, w3.Code)
	})

	t.Run("POST /api/auth/refresh rejects invalid token", func(t *testing.T) {
		body := dto.RefreshRequest{RefreshToken: "not-a-real-token"}
		w := doRequest(ts, http.MethodPost, "/api/auth/refresh", body, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("POST /api/auth/logout invalidates refresh tokens", func(t *testing.T) {
		logoutUser := createTestUser(t, ts)

		w := doRequest(ts, http.MethodPost, "/api/auth/logout", nil, logoutUser.AccessToken)
		assert.Equal(t, http.StatusNoContent, w.Code)

		// Refresh token is invalidated
		w2 := doRequest(ts, http.MethodPost, "/api/auth/refresh",
			dto.RefreshRequest{RefreshToken: logoutUser.RefreshToken}, "")
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("DELETE /api/auth/account removes user", func(t *testing.T) {
		deleteUser := createTestUser(t, ts)

		w := doRequest(ts, http.MethodDelete, "/api/auth/account", nil, deleteUser.AccessToken)
		assert.Equal(t, http.StatusNoContent, w.Code)

		// User no longer exists
		w2 := doRequest(ts, http.MethodGet, "/api/auth/me", nil, deleteUser.AccessToken)
		assert.Equal(t, http.StatusNotFound, w2.Code)
	})
}

// ---------------------------------------------------------------------------
// 4. Event CRUD
// ---------------------------------------------------------------------------

func TestIntegration_EventCRUD(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	var createdEventID string

	t.Run("POST /api/events creates event", func(t *testing.T) {
		body := dto.CreateEventRequest{
			Title:    "Team Meeting",
			StartAt:  "2024-03-15T10:00:00+09:00",
			EndAt:    "2024-03-15T11:00:00+09:00",
			Timezone: "Asia/Seoul",
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		assert.Equal(t, http.StatusCreated, w.Code)

		resp := parseJSON[dto.EventResponse](t, w)
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, "Team Meeting", resp.Title)
		assert.Equal(t, "Asia/Seoul", resp.Timezone)
		assert.Equal(t, false, resp.AllDay)
		assert.Equal(t, []int64{180}, resp.ReminderOffsets) // default 3 hours
		createdEventID = resp.ID
	})

	t.Run("GET /api/events/:id returns event", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/events/"+createdEventID, nil, user.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.EventResponse](t, w)
		assert.Equal(t, createdEventID, resp.ID)
		assert.Equal(t, "Team Meeting", resp.Title)
	})

	t.Run("GET /api/events lists events by date range", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet,
			"/api/events?start=2024-03-01T00:00:00Z&end=2024-04-01T00:00:00Z",
			nil, user.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.EventListResponse](t, w)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "Team Meeting", resp.Events[0].Title)
	})

	t.Run("GET /api/events returns empty for out-of-range", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet,
			"/api/events?start=2025-01-01T00:00:00Z&end=2025-02-01T00:00:00Z",
			nil, user.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.EventListResponse](t, w)
		assert.Equal(t, 0, resp.Count)
	})

	t.Run("PUT /api/events/:id updates event", func(t *testing.T) {
		body := dto.UpdateEventRequest{
			Title:    "Updated Meeting",
			StartAt:  "2024-03-15T14:00:00+09:00",
			EndAt:    "2024-03-15T15:00:00+09:00",
			Timezone: "Asia/Seoul",
		}
		w := doRequest(ts, http.MethodPut, "/api/events/"+createdEventID, body, user.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.EventResponse](t, w)
		assert.Equal(t, "Updated Meeting", resp.Title)
	})

	t.Run("DELETE /api/events/:id removes event", func(t *testing.T) {
		w := doRequest(ts, http.MethodDelete, "/api/events/"+createdEventID, nil, user.AccessToken)
		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify it's gone
		w2 := doRequest(ts, http.MethodGet, "/api/events/"+createdEventID, nil, user.AccessToken)
		assert.Equal(t, http.StatusNotFound, w2.Code)
	})
}

// ---------------------------------------------------------------------------
// 5. Event Validation
// ---------------------------------------------------------------------------

func TestIntegration_EventValidation(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	t.Run("end before start returns 400", func(t *testing.T) {
		body := dto.CreateEventRequest{
			Title:   "Bad Range",
			StartAt: "2024-03-15T12:00:00Z",
			EndAt:   "2024-03-15T10:00:00Z",
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid timezone returns 400", func(t *testing.T) {
		body := dto.CreateEventRequest{
			Title:    "Bad TZ",
			StartAt:  "2024-03-15T10:00:00Z",
			EndAt:    "2024-03-15T11:00:00Z",
			Timezone: "Invalid/Timezone",
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid date format returns 400", func(t *testing.T) {
		body := map[string]any{
			"title":   "Bad Date",
			"startAt": "not-a-date",
			"endAt":   "2024-03-15T11:00:00Z",
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("nonexistent event returns 404", func(t *testing.T) {
		fakeID := uuid.New().String()
		w := doRequest(ts, http.MethodGet, "/api/events/"+fakeID, nil, user.AccessToken)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid UUID in path returns 400", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/events/not-a-uuid", nil, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing start/end query returns 400", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/events", nil, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("whitespace-only title returns 400", func(t *testing.T) {
		body := map[string]any{
			"title":   "   ",
			"startAt": "2024-03-15T10:00:00Z",
			"endAt":   "2024-03-15T11:00:00Z",
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ---------------------------------------------------------------------------
// 6. User Isolation
// ---------------------------------------------------------------------------

func TestIntegration_UserIsolation(t *testing.T) {
	ts := setupTestServer(t)
	userA := createTestUser(t, ts)
	userB := createTestUser(t, ts)

	// User A creates an event
	body := dto.CreateEventRequest{
		Title:   "User A Event",
		StartAt: "2024-03-15T10:00:00Z",
		EndAt:   "2024-03-15T11:00:00Z",
	}
	w := doRequest(ts, http.MethodPost, "/api/events", body, userA.AccessToken)
	require.Equal(t, http.StatusCreated, w.Code)
	eventResp := parseJSON[dto.EventResponse](t, w)
	eventID := eventResp.ID

	t.Run("user B cannot read user A event", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet, "/api/events/"+eventID, nil, userB.AccessToken)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("user B cannot update user A event", func(t *testing.T) {
		updateBody := dto.UpdateEventRequest{
			Title:   "Hacked",
			StartAt: "2024-03-15T10:00:00Z",
			EndAt:   "2024-03-15T11:00:00Z",
		}
		w := doRequest(ts, http.MethodPut, "/api/events/"+eventID, updateBody, userB.AccessToken)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("user B cannot delete user A event", func(t *testing.T) {
		w := doRequest(ts, http.MethodDelete, "/api/events/"+eventID, nil, userB.AccessToken)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("user B list does not include user A events", func(t *testing.T) {
		w := doRequest(ts, http.MethodGet,
			"/api/events?start=2024-03-01T00:00:00Z&end=2024-04-01T00:00:00Z",
			nil, userB.AccessToken)
		assert.Equal(t, http.StatusOK, w.Code)

		resp := parseJSON[dto.EventListResponse](t, w)
		assert.Equal(t, 0, resp.Count)
	})
}

// ---------------------------------------------------------------------------
// 7. Event with All Fields
// ---------------------------------------------------------------------------

func TestIntegration_EventAllFields(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	locName := "Cafe Sunset"
	locAddr := "123 Main St, Seoul"
	locLat := 37.5665
	locLng := 126.9780
	notes := "Bring laptop"

	body := dto.CreateEventRequest{
		Title:           "Full Event",
		StartAt:         "2024-06-01T09:00:00+09:00",
		EndAt:           "2024-06-01T17:00:00+09:00",
		AllDay:          false,
		Timezone:        "Asia/Seoul",
		LocationName:    &locName,
		LocationAddress: &locAddr,
		LocationLat:     &locLat,
		LocationLng:     &locLng,
		ReminderOffsets: []int64{10, 30, 60},
		Notes:           &notes,
	}

	w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
	require.Equal(t, http.StatusCreated, w.Code)

	resp := parseJSON[dto.EventResponse](t, w)
	assert.Equal(t, "Full Event", resp.Title)
	assert.Equal(t, false, resp.AllDay)
	assert.Equal(t, "Asia/Seoul", resp.Timezone)
	require.NotNil(t, resp.LocationName)
	assert.Equal(t, "Cafe Sunset", *resp.LocationName)
	require.NotNil(t, resp.LocationAddress)
	assert.Equal(t, "123 Main St, Seoul", *resp.LocationAddress)
	require.NotNil(t, resp.LocationLat)
	assert.InDelta(t, 37.5665, *resp.LocationLat, 0.0001)
	require.NotNil(t, resp.LocationLng)
	assert.InDelta(t, 126.9780, *resp.LocationLng, 0.0001)
	assert.Equal(t, []int64{10, 30, 60}, resp.ReminderOffsets)
	require.NotNil(t, resp.Notes)
	assert.Equal(t, "Bring laptop", *resp.Notes)

	// Verify round-trip: GET returns the same data
	getW := doRequest(ts, http.MethodGet, "/api/events/"+resp.ID, nil, user.AccessToken)
	require.Equal(t, http.StatusOK, getW.Code)

	getResp := parseJSON[dto.EventResponse](t, getW)
	assert.Equal(t, resp.Title, getResp.Title)
	assert.Equal(t, *resp.LocationName, *getResp.LocationName)
	assert.Equal(t, resp.ReminderOffsets, getResp.ReminderOffsets)
}

// ---------------------------------------------------------------------------
// 8. Voice Parsing
// ---------------------------------------------------------------------------

func TestIntegration_VoiceParsing(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	t.Run("returns 502 when AI service unavailable", func(t *testing.T) {
		body := dto.ParseVoiceRequest{Text: "Meeting tomorrow at 3pm"}
		w := doRequest(ts, http.MethodPost, "/api/events/parse-voice", body, user.AccessToken)
		assert.Equal(t, http.StatusBadGateway, w.Code)

		problem := parseJSON[response.ProblemDetail](t, w)
		assert.Contains(t, problem.Detail, "AI service unavailable")
	})

	t.Run("rejects empty text", func(t *testing.T) {
		body := dto.ParseVoiceRequest{Text: "   "}
		w := doRequest(ts, http.MethodPost, "/api/events/parse-voice", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		body := dto.ParseVoiceRequest{Text: "Meeting tomorrow"}
		w := doRequest(ts, http.MethodPost, "/api/events/parse-voice", body, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
