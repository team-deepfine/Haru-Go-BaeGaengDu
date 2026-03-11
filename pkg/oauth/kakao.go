package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	kakaoTokenURL   = "https://kauth.kakao.com/oauth/token"
	kakaoUserMeURL  = "https://kapi.kakao.com/v2/user/me"
)

// KakaoUserInfo holds the user info retrieved from Kakao APIs.
type KakaoUserInfo struct {
	Sub          string  // Kakao user ID (int64 → string)
	Email        *string
	Nickname     *string
	ProfileImage *string
}

// KakaoClient exchanges Kakao authorization codes for user information.
type KakaoClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

// NewKakaoClient creates a new Kakao OAuth client.
func NewKakaoClient(clientID, clientSecret, redirectURI string) *KakaoClient {
	return &KakaoClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{Timeout: 10 * 1e9}, // 10 seconds
	}
}

// ExchangeAndGetUser exchanges an authorization code for a Kakao access token,
// then fetches user info from the Kakao API.
func (k *KakaoClient) ExchangeAndGetUser(ctx context.Context, code string) (*KakaoUserInfo, error) {
	accessToken, err := k.exchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	return k.getUserInfo(ctx, accessToken)
}

// exchangeCode exchanges an authorization code for a Kakao access token.
func (k *KakaoClient) exchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {k.clientID},
		"redirect_uri": {k.redirectURI},
		"code":         {code},
	}
	if k.clientSecret != "" {
		data.Set("client_secret", k.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kakaoTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("kakao token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kakao token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp kakaoTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in kakao response")
	}

	return tokenResp.AccessToken, nil
}

// getUserInfo fetches user profile from the Kakao user info API.
func (k *KakaoClient) getUserInfo(ctx context.Context, accessToken string) (*KakaoUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kakaoUserMeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create user info request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kakao user info request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kakao user info endpoint returned status %d", resp.StatusCode)
	}

	var userResp kakaoUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("decode user info response: %w", err)
	}

	info := &KakaoUserInfo{
		Sub: strconv.FormatInt(userResp.ID, 10),
	}

	if userResp.KakaoAccount != nil {
		if userResp.KakaoAccount.Email != "" && userResp.KakaoAccount.IsEmailVerified {
			info.Email = &userResp.KakaoAccount.Email
		}
		if userResp.KakaoAccount.Profile != nil {
			if userResp.KakaoAccount.Profile.Nickname != "" {
				info.Nickname = &userResp.KakaoAccount.Profile.Nickname
			}
			if userResp.KakaoAccount.Profile.ProfileImageURL != "" {
				info.ProfileImage = &userResp.KakaoAccount.Profile.ProfileImageURL
			}
		}
	}

	return info, nil
}

// Kakao API response types (unexported).

type kakaoTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type kakaoUserResponse struct {
	ID           int64         `json:"id"`
	KakaoAccount *kakaoAccount `json:"kakao_account"`
}

type kakaoAccount struct {
	Profile         *kakaoProfile `json:"profile"`
	Email           string        `json:"email"`
	IsEmailVerified bool          `json:"is_email_verified"`
}

type kakaoProfile struct {
	Nickname        string `json:"nickname"`
	ProfileImageURL string `json:"profile_image_url"`
}
