package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ==== Konstanta GitHub API ====

const (
	GitHubOAuthBaseURL = "https://github.com/login/oauth"
	GitHubAPIBaseURL   = "https://api.github.com"

	// Scope minimal yang dibutuhkan untuk membaca repo & mengelola webhook
	DefaultOAuthScopes = "read:user repo admin:repo_hook"
)

// ==== Tipe Data ====

// GitHubTokenResponse adalah respons dari exchange OAuth.
type GitHubTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
	RefreshTokenExp  int    `json:"refresh_token_expires_in,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// GitHubRepository adalah subset data repo yang relevan untuk ditampilkan di GUI/CLI.
type GitHubRepository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	UpdatedAt     string `json:"updated_at"`
}

// ListRepositoriesOptions adalah opsi paginasi & filter.
type ListRepositoriesOptions struct {
	Type      string // all | owner | public | private | member (default: all)
	Sort      string // created | updated | pushed | full_name (default: full_name)
	Direction string // asc | desc (default: asc jika full_name)
	PerPage   int    // max 100 (default: 30)
	Page      int    // default: 1
}

// WebhookConfig adalah konfigurasi webhook yang dikirim ke GitHub.
type WebhookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"` // "json" atau "form"
	Secret      string `json:"secret,omitempty"`
	InsecureSSL string `json:"insecure_ssl,omitempty"` // "0" atau "1"
}

// CreateWebhookRequest adalah body untuk POST /repos/{owner}/{repo}/hooks.
type CreateWebhookRequest struct {
	Name   string        `json:"name"`   // harus "web"
	Active bool          `json:"active"` // default true
	Events []string      `json:"events"` // ["push", "pull_request", ...]
	Config WebhookConfig `json:"config"`
}

// GitHubWebhook adalah respons dari GitHub saat webhook berhasil dibuat.
type GitHubWebhook struct {
	ID     int64         `json:"id"`
	Name   string        `json:"name"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config WebhookConfig `json:"config"`
	URL    string        `json:"url"`
}

// SetupWebhookResult adalah hasil akhir SetupAutoWebhook.
type SetupWebhookResult struct {
	WebhookID int64  `json:"webhook_id"`
	HTMLURL   string `json:"html_url"` // URL webhook di GitHub
	HookURL   string `json:"hook_url"` // URL endpoint server kita
}

// ==== Service ====

// GitHubService membungkus operasi OAuth, repo, dan webhook.
type GitHubService struct {
	clientID       string
	clientSecret   string
	webhookSecret  string
	webhookBaseURL string // contoh: https://api.domainanda.com
	httpClient     *http.Client
}

// GitHubUser adalah subset profil user GitHub.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// NewGitHubService membuat instance GitHubService.
func NewGitHubService(clientID, clientSecret, webhookSecret, webhookBaseURL string) *GitHubService {
	return &GitHubService{
		clientID:       clientID,
		clientSecret:   clientSecret,
		webhookSecret:  webhookSecret,
		webhookBaseURL: strings.TrimRight(webhookBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ==== 1. ExchangeOAuthToken ====

// ExchangeOAuthToken menukar authorization code dengan access token.
// Endpoint: POST https://github.com/login/oauth/access_token
func (s *GitHubService) ExchangeOAuthToken(ctx context.Context, code string) (*GitHubTokenResponse, error) {
	if code == "" {
		return nil, fmt.Errorf("authorization code kosong")
	}

	payload := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		GitHubOAuthBaseURL+"/access_token",
		strings.NewReader(payload.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request token: %w", err)
	}

	// Wajib: minta respons JSON, bukan form-encoded default
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request token gagal: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal baca respons token: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "list repos"}
	}

	var tokenResp GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("gagal decode respons token: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("GitHub OAuth error: %s — %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("access token kosong dalam respons GitHub")
	}

	return &tokenResp, nil
}

// ==== 2. FetchUserRepositories ====

// FetchUserRepositories mengambil daftar repo milik user yang terautentikasi.
// Endpoint: GET https://api.github.com/user/repos
func (s *GitHubService) FetchUserRepositories(
	ctx context.Context,
	accessToken string,
	opts *ListRepositoriesOptions,
) ([]GitHubRepository, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token kosong")
	}

	// Default opsi
	if opts == nil {
		opts = &ListRepositoriesOptions{}
	}
	if opts.Type == "" {
		opts.Type = "all"
	}
	if opts.Sort == "" {
		opts.Sort = "full_name"
	}
	if opts.PerPage <= 0 || opts.PerPage > 100 {
		opts.PerPage = 30
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	u, _ := url.Parse(GitHubAPIBaseURL + "/user/repos")
	q := u.Query()
	q.Set("type", opts.Type)
	q.Set("sort", opts.Sort)
	q.Set("per_page", fmt.Sprintf("%d", opts.PerPage))
	q.Set("page", fmt.Sprintf("%d", opts.Page))
	if opts.Direction != "" {
		q.Set("direction", opts.Direction)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request repos: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request repos gagal: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal baca respons repos: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "list repos"}
	}

	var repos []GitHubRepository
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("gagal decode repos: %w", err)
	}

	return repos, nil
}

// ==== 3. SetupAutoWebhook ====

// SetupAutoWebhook mendaftarkan endpoint server kita sebagai webhook di repo user.
// Endpoint: POST https://api.github.com/repos/{owner}/{repo}/hooks
//
// fullRepoName harus dalam format "owner/repo", contoh: "octocat/Hello-World".
// events default: ["push", "pull_request"] jika kosong.
func (s *GitHubService) SetupAutoWebhook(
	ctx context.Context,
	accessToken, fullRepoName string,
	events []string,
) (*SetupWebhookResult, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token kosong")
	}

	parts := strings.SplitN(fullRepoName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("format repo tidak valid: '%s' (harus owner/repo)", fullRepoName)
	}
	owner, repo := parts[0], parts[1]

	// Default events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}

	hookURL := s.WebhookURL()

	payload := CreateWebhookRequest{
		Name:   "web",
		Active: true,
		Events: events,
		Config: WebhookConfig{
			URL:         hookURL,
			ContentType: "json",
			Secret:      s.webhookSecret,
			InsecureSSL: "0",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gagal marshal webhook payload: %w", err)
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/hooks", GitHubAPIBaseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request webhook: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request webhook gagal: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal baca respons webhook: %w", err)
	}

	// Handle idempotensi: jika webhook sudah ada, GitHub mengembalikan 422
	if resp.StatusCode == http.StatusUnprocessableEntity {
		// Coba cari webhook existing dengan URL yang sama
		existing, findErr := s.findExistingWebhook(ctx, accessToken, owner, repo, hookURL)
		if findErr == nil && existing != nil {
			return &SetupWebhookResult{
				WebhookID: existing.ID,
				HTMLURL:   fmt.Sprintf("https://github.com/%s/%s/settings/hooks/%d", owner, repo, existing.ID),
				HookURL:   hookURL,
			}, nil
		}
		return nil, fmt.Errorf("webhook sudah ada atau konflik: %s", string(body))
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "list repos"}
	}

	var webhook GitHubWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return nil, fmt.Errorf("gagal decode webhook response: %w", err)
	}

	return &SetupWebhookResult{
		WebhookID: webhook.ID,
		HTMLURL:   fmt.Sprintf("https://github.com/%s/%s/settings/hooks/%d", owner, repo, webhook.ID),
		HookURL:   hookURL,
	}, nil
}

// findExistingWebhook mencari webhook dengan URL yang sama (untuk idempotensi).
func (s *GitHubService) findExistingWebhook(
	ctx context.Context, accessToken, owner, repo, targetURL string,
) (*GitHubWebhook, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/hooks", GitHubAPIBaseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list webhooks HTTP %d", resp.StatusCode)
	}

	var hooks []GitHubWebhook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return nil, err
	}

	for _, h := range hooks {
		if h.Config.URL == targetURL {
			return &h, nil
		}
	}
	return nil, nil
}

// ==== Helper: Hapus Webhook (opsional) ====

// DeleteWebhook menghapus webhook dari repo.
// Endpoint: DELETE /repos/{owner}/{repo}/hooks/{hook_id}
func (s *GitHubService) DeleteWebhook(
	ctx context.Context,
	accessToken, fullRepoName string,
	hookID int64,
) error {
	parts := strings.SplitN(fullRepoName, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("format repo tidak valid: %s", fullRepoName)
	}
	owner, repo := parts[0], parts[1]

	apiURL := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", GitHubAPIBaseURL, owner, repo, hookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "list repos"}
	}
	return nil
}

// ==== Helper OAuth & verifikasi ====

// WebhookURL adalah endpoint server kita yang didaftarkan ke GitHub.
func (s *GitHubService) WebhookURL() string {
	return s.webhookBaseURL + "/api/webhook/github"
}

// AuthURL membangun URL redirect login GitHub (authorization code flow).
func (s *GitHubService) AuthURL(redirectURI, state string) string {
	q := url.Values{
		"client_id":    {s.clientID},
		"redirect_uri": {redirectURI},
		"scope":        {DefaultOAuthScopes},
		"state":        {state},
	}
	return GitHubOAuthBaseURL + "/authorize?" + q.Encode()
}

// Configured true jika client id & secret sudah diisi.
func (s *GitHubService) Configured() bool {
	return s.clientID != "" && s.clientSecret != ""
}

// FetchUser mengambil profil user yang terautentikasi (GET /user).
func (s *GitHubService) FetchUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIBaseURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request user gagal: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "list repos"}
	}
	var u GitHubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("gagal decode user: %w", err)
	}
	return &u, nil
}

// VerifyWebhookSignature memvalidasi header X-Hub-Signature-256 terhadap body mentah.
// Jika webhook secret kosong, verifikasi dilewati hanya saat dikonfigurasi demikian (return false = tolak).
func (s *GitHubService) VerifyWebhookSignature(body []byte, signatureHeader string) bool {
	if s.webhookSecret == "" {
		return false // secret wajib; tolak webhook tak terautentikasi
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, prefix))
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(s.webhookSecret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

// GitHubAPIError mengembalikan status HTTP dari GitHub agar caller bisa
// membedakan 401 (Bad credentials) dari error lain.
type GitHubAPIError struct {
	StatusCode int
	Body       string
	Operation  string
}

func (e *GitHubAPIError) Error() string {
	return fmt.Sprintf("%s HTTP %d: %s", e.Operation, e.StatusCode, e.Body)
}

// ==== RefreshAccessToken ====
//
// GitHub OAuth refresh flow (hanya berlaku bila app punya refresh token,
// umumnya saat "expiring user tokens" diaktifkan di GitHub App settings).
// Endpoint: POST https://github.com/login/oauth/access_token
// grant_type=refresh_token
func (s *GitHubService) RefreshAccessToken(ctx context.Context, refreshToken string) (*GitHubTokenResponse, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token kosong")
	}
	payload := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		GitHubOAuthBaseURL+"/access_token", strings.NewReader(payload.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "paas-backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &GitHubAPIError{StatusCode: resp.StatusCode, Body: string(body), Operation: "refresh token"}
	}
	var tok GitHubTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, err
	}
	if tok.Error != "" {
		return nil, &GitHubAPIError{
			StatusCode: http.StatusUnauthorized,
			Body:       tok.Error + " — " + tok.ErrorDescription,
			Operation:  "refresh token",
		}
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("access token kosong setelah refresh")
	}
	return &tok, nil
}
