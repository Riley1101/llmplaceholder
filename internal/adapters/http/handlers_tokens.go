package http

import (
	"fmt"
	"net/http"
	"strings"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
)

type uiTokensData struct {
	Tokens []models.APIToken
}

type uiTokenCreatedData struct {
	Token  string
	Tokens []models.APIToken
}

// GET /ui/api-tokens
func HandleUIGetTokens(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, _ := dbManager.ListAPITokens(UserFromContext(r).ID)
		renderPartial(w, "api-tokens-list", uiTokensData{Tokens: tokens})
	}
}

// POST /ui/api-tokens  form: name
func HandleUICreateToken(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			name = "API Token"
		}
		plain, err := dbManager.CreateAPIToken(user.ID, name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span class="text-red-400 text-[12px]">✗ %s</span>`, err.Error())
			return
		}
		tokens, _ := dbManager.ListAPITokens(user.ID)
		renderPartial(w, "api-token-created", uiTokenCreatedData{Token: plain, Tokens: tokens})
	}
}

// DELETE /ui/api-tokens/{id}
func HandleUIDeleteToken(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbManager.DeleteAPIToken(r.PathValue("id"), UserFromContext(r).ID)
		w.WriteHeader(http.StatusOK)
	}
}
