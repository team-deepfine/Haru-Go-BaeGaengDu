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
	engine          *gin.Engine
	db              *gorm.DB
	jwtManager      *jwtpkg.Manager
	userRepo        repository.UserRepository
	tokenRepo       repository.TokenRepository
	eventRepo       repository.EventRepository
	notifRepo       repository.NotificationRepository
	deviceTokenRepo repository.DeviceTokenRepository
}

type testUser struct {
	ID           uuid.UUID
	AccessToken  string
	RefreshToken string
}

// testApplePrivateKeyPEM is a test-only ES256 private key (not used in production).
const testApplePrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQggNPpkAwDVkvZw6H1
sOZ0HU1bCMeoQ1b1+OnuG8AxbU6hRANCAAQ85HzhTQNWkkyZXtdZMV2B96l+paFE
XM+YSpZkqDqkJoOYd5TYGa7iDC5iDFxWYeyIpt80lnMX/0KVH0ipb2Sf
-----END PRIVATE KEY-----`

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

	err = db.AutoMigrate(&model.User{}, &model.RefreshToken{}, &model.Event{}, &model.Notification{}, &model.DeviceToken{})
	require.NoError(t, err)

	jwtManager := jwtpkg.NewManager("integration-test-secret", time.Hour, 720*time.Hour)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	eventRepo := repository.NewEventRepository(db)

	appleClient, err := oauth.NewAppleClient(
		"test-client-id", "test-team-id", "test-key-id",
		testApplePrivateKeyPEM, "",
	)
	require.NoError(t, err)
	kakaoClient := oauth.NewKakaoClient("test-kakao-id", "", "")
	authSvc := service.NewAuthService(userRepo, tokenRepo, jwtManager, appleClient, kakaoClient)
	authHandler := handler.NewAuthHandler(authSvc)

	notifRepo := repository.NewNotificationRepository(db)
	notifScheduler := service.NewNotificationScheduler(notifRepo)
	eventSvc := service.NewEventService(eventRepo, service.WithNotificationScheduler(notifScheduler))
	eventHandler := handler.NewEventHandler(eventSvc)

	voiceHandler := handler.NewVoiceHandler(&noopVoiceParsingService{})

	deviceTokenRepo := repository.NewDeviceTokenRepository(db)
	deviceTokenSvc := service.NewDeviceTokenService(deviceTokenRepo)
	deviceTokenHandler := handler.NewDeviceTokenHandler(deviceTokenSvc)

	engine := router.New(jwtManager, authHandler, eventHandler, voiceHandler, deviceTokenHandler)

	return &testServer{
		engine:          engine,
		db:              db,
		jwtManager:      jwtManager,
		userRepo:        userRepo,
		tokenRepo:       tokenRepo,
		eventRepo:       eventRepo,
		notifRepo:       notifRepo,
		deviceTokenRepo: deviceTokenRepo,
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

// ---------------------------------------------------------------------------
// 9. Device Token API
// ---------------------------------------------------------------------------

func TestIntegration_DeviceToken(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	t.Run("POST /api/devices registers token", func(t *testing.T) {
		body := dto.RegisterDeviceTokenRequest{Token: "fcm-token-abc123"}
		w := doRequest(ts, http.MethodPost, "/api/devices", body, user.AccessToken)
		assert.Equal(t, http.StatusCreated, w.Code)

		resp := parseJSON[dto.DeviceTokenResponse](t, w)
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, "fcm-token-abc123", resp.Token)
		assert.NotEmpty(t, resp.CreatedAt)
	})

	t.Run("POST /api/devices upserts same token", func(t *testing.T) {
		body := dto.RegisterDeviceTokenRequest{Token: "fcm-token-upsert"}
		w1 := doRequest(ts, http.MethodPost, "/api/devices", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w1.Code)

		// Register the same token again — should not error
		w2 := doRequest(ts, http.MethodPost, "/api/devices", body, user.AccessToken)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("POST /api/devices upserts token ownership on account switch", func(t *testing.T) {
		userB := createTestUser(t, ts)

		body := dto.RegisterDeviceTokenRequest{Token: "fcm-shared-device-token"}
		w1 := doRequest(ts, http.MethodPost, "/api/devices", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w1.Code)

		// Different user registers the same token (app reinstall / account switch)
		w2 := doRequest(ts, http.MethodPost, "/api/devices", body, userB.AccessToken)
		assert.Equal(t, http.StatusCreated, w2.Code)

		// Token should now belong to userB
		tokens, err := ts.deviceTokenRepo.FindByUserID(context.Background(), userB.ID)
		require.NoError(t, err)
		found := false
		for _, tok := range tokens {
			if tok.Token == "fcm-shared-device-token" {
				found = true
			}
		}
		assert.True(t, found, "token should belong to userB after upsert")
	})

	t.Run("POST /api/devices rejects empty token", func(t *testing.T) {
		body := map[string]any{"token": ""}
		w := doRequest(ts, http.MethodPost, "/api/devices", body, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /api/devices rejects missing body", func(t *testing.T) {
		w := doRequest(ts, http.MethodPost, "/api/devices", nil, user.AccessToken)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /api/devices rejects unauthenticated", func(t *testing.T) {
		body := dto.RegisterDeviceTokenRequest{Token: "fcm-token-noauth"}
		w := doRequest(ts, http.MethodPost, "/api/devices", body, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("DELETE /api/devices removes token", func(t *testing.T) {
		// First register
		regBody := dto.RegisterDeviceTokenRequest{Token: "fcm-token-to-delete"}
		w := doRequest(ts, http.MethodPost, "/api/devices", regBody, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)

		// Then delete
		delBody := dto.UnregisterDeviceTokenRequest{Token: "fcm-token-to-delete"}
		w2 := doRequest(ts, http.MethodDelete, "/api/devices", delBody, user.AccessToken)
		assert.Equal(t, http.StatusNoContent, w2.Code)
	})

	t.Run("DELETE /api/devices returns 404 for unknown token", func(t *testing.T) {
		body := dto.UnregisterDeviceTokenRequest{Token: "nonexistent-token"}
		w := doRequest(ts, http.MethodDelete, "/api/devices", body, user.AccessToken)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("DELETE /api/devices rejects unauthenticated", func(t *testing.T) {
		body := dto.UnregisterDeviceTokenRequest{Token: "some-token"}
		w := doRequest(ts, http.MethodDelete, "/api/devices", body, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// ---------------------------------------------------------------------------
// 10. Notification Scheduling via Event CRUD
// ---------------------------------------------------------------------------

func TestIntegration_NotificationScheduling(t *testing.T) {
	ts := setupTestServer(t)
	user := createTestUser(t, ts)

	t.Run("creating event schedules notifications", func(t *testing.T) {
		// Create an event in the future with specific reminder offsets
		body := dto.CreateEventRequest{
			Title:           "Future Meeting",
			StartAt:         time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			EndAt:           time.Now().Add(25 * time.Hour).UTC().Format(time.RFC3339),
			Timezone:        "UTC",
			ReminderOffsets: []int64{10, 60},
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)

		resp := parseJSON[dto.EventResponse](t, w)
		eventID, err := uuid.Parse(resp.ID)
		require.NoError(t, err)

		// Check that notifications were created
		notifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(notifs), "should create one notification per reminder offset")

		// Verify offsets
		offsets := make(map[int]bool)
		for _, n := range notifs {
			offsets[n.OffsetMin] = true
			assert.Equal(t, user.ID, n.UserID)
			assert.False(t, n.Sent)
		}
		assert.True(t, offsets[10], "should have 10-min offset")
		assert.True(t, offsets[60], "should have 60-min offset")
	})

	t.Run("creating event with past reminders skips them", func(t *testing.T) {
		// Event starts in 5 minutes — a 10-min reminder would be in the past
		body := dto.CreateEventRequest{
			Title:           "Soon Meeting",
			StartAt:         time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
			EndAt:           time.Now().Add(65 * time.Minute).UTC().Format(time.RFC3339),
			Timezone:        "UTC",
			ReminderOffsets: []int64{0, 10, 60},
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)

		resp := parseJSON[dto.EventResponse](t, w)
		eventID, err := uuid.Parse(resp.ID)
		require.NoError(t, err)

		notifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)

		// Only offset=0 (at start time, 5min from now) should survive
		// offset=10 would be 5min ago, offset=60 would be 55min ago
		for _, n := range notifs {
			assert.True(t, n.NotifyAt.After(time.Now().Add(-1*time.Second)),
				"notification should not be in the past: offset=%d, notifyAt=%s", n.OffsetMin, n.NotifyAt)
		}
	})

	t.Run("updating event reschedules notifications", func(t *testing.T) {
		// Create event
		body := dto.CreateEventRequest{
			Title:           "Reschedulable",
			StartAt:         time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
			EndAt:           time.Now().Add(49 * time.Hour).UTC().Format(time.RFC3339),
			Timezone:        "UTC",
			ReminderOffsets: []int64{10},
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseJSON[dto.EventResponse](t, w)
		eventID, err := uuid.Parse(resp.ID)
		require.NoError(t, err)

		// Verify initial notification
		notifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		require.Equal(t, 1, len(notifs))
		oldNotifyAt := notifs[0].NotifyAt

		// Update with different time and offsets
		updateBody := dto.UpdateEventRequest{
			Title:           "Reschedulable Updated",
			StartAt:         time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			EndAt:           time.Now().Add(73 * time.Hour).UTC().Format(time.RFC3339),
			Timezone:        "UTC",
			ReminderOffsets: []int64{30, 60},
		}
		w2 := doRequest(ts, http.MethodPut, "/api/events/"+resp.ID, updateBody, user.AccessToken)
		require.Equal(t, http.StatusOK, w2.Code)

		// Check notifications were replaced
		newNotifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(newNotifs), "should have 2 new notifications")
		for _, n := range newNotifs {
			assert.NotEqual(t, oldNotifyAt, n.NotifyAt, "notification time should have changed")
		}
	})

	t.Run("deleting event cancels notifications", func(t *testing.T) {
		body := dto.CreateEventRequest{
			Title:           "To Be Deleted",
			StartAt:         time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
			EndAt:           time.Now().Add(49 * time.Hour).UTC().Format(time.RFC3339),
			Timezone:        "UTC",
			ReminderOffsets: []int64{10, 30},
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseJSON[dto.EventResponse](t, w)
		eventID, err := uuid.Parse(resp.ID)
		require.NoError(t, err)

		// Verify notifications exist
		notifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		require.Equal(t, 2, len(notifs))

		// Delete event
		w2 := doRequest(ts, http.MethodDelete, "/api/events/"+resp.ID, nil, user.AccessToken)
		require.Equal(t, http.StatusNoContent, w2.Code)

		// Notifications should be gone
		notifs, err = ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		assert.Equal(t, 0, len(notifs), "notifications should be deleted with event")
	})

	t.Run("default reminder offset is 180 min", func(t *testing.T) {
		body := dto.CreateEventRequest{
			Title:    "Default Reminder",
			StartAt:  time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
			EndAt:    time.Now().Add(49 * time.Hour).UTC().Format(time.RFC3339),
			Timezone: "UTC",
			// No ReminderOffsets — should default to [180]
		}
		w := doRequest(ts, http.MethodPost, "/api/events", body, user.AccessToken)
		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseJSON[dto.EventResponse](t, w)
		eventID, err := uuid.Parse(resp.ID)
		require.NoError(t, err)

		notifs, err := ts.notifRepo.FindByEventID(context.Background(), eventID)
		require.NoError(t, err)
		require.Equal(t, 1, len(notifs))
		assert.Equal(t, 180, notifs[0].OffsetMin, "default offset should be 180 minutes")
	})
}
