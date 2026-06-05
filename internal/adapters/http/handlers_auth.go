package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
)

const sessionCookieName = "session_id"
const stateCookieName = "oauth_state"

func githubClientID() string     { return os.Getenv("GITHUB_CLIENT_ID") }
func githubClientSecret() string { return os.Getenv("GITHUB_CLIENT_SECRET") }
func isProduction() bool         { return os.Getenv("ENV") == "production" }

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type githubTokenResp struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type githubUserResp struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmailResp struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r) != nil {
			http.Redirect(w, r, "/playground", http.StatusFound)
			return
		}
		render(w, "login", pageData{})
	}
}

func HandleGithubLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if githubClientID() == "" {
			http.Error(w, "GitHub OAuth not configured: set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET", http.StatusServiceUnavailable)
			return
		}
		state := randomHex(16)
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    state,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			Secure:   isProduction(),
			SameSite: http.SameSiteLaxMode,
		})
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&scope=read%%3Auser,user%%3Aemail&state=%s",
			url.QueryEscape(githubClientID()),
			url.QueryEscape(state),
		)
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func HandleGithubCallback(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateCookie, err := r.Cookie(stateCookieName)
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: stateCookieName, MaxAge: -1, Path: "/"})

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		token, err := exchangeGithubCode(code)
		if err != nil {
			http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		ghUser, err := githubGetUser(token)
		if err != nil {
			http.Error(w, "failed to fetch GitHub user", http.StatusInternalServerError)
			return
		}

		if ghUser.Email == "" {
			ghUser.Email, _ = githubGetPrimaryEmail(token)
		}

		user := &models.User{
			GithubID:  ghUser.ID,
			Login:     ghUser.Login,
			Name:      ghUser.Name,
			Email:     ghUser.Email,
			AvatarURL: ghUser.AvatarURL,
		}

		if err := dbManager.UpsertUser(user); err != nil {
			http.Error(w, "failed to save user", http.StatusInternalServerError)
			return
		}

		sessionID := randomHex(32)
		if err := dbManager.CreateSession(sessionID, user.ID); err != nil {
			http.Error(w, "failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   7 * 24 * 3600,
			HttpOnly: true,
			Secure:   isProduction(),
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/playground", http.StatusFound)
	}
}

func HandleLogout(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			dbManager.DeleteSession(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, MaxAge: -1, Path: "/"})
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func exchangeGithubCode(code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", githubClientID())
	form.Set("client_secret", githubClientSecret())
	form.Set("code", code)

	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var t githubTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	if t.Error != "" {
		return "", fmt.Errorf("%s", t.Error)
	}
	return t.AccessToken, nil
}

func githubGetUser(token string) (*githubUserResp, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var u githubUserResp
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func githubGetPrimaryEmail(token string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []githubEmailResp
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}
