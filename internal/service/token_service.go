package service

import (
	"context"
	"errors"
	"time"

	"W-dev/internal/repository"
	"W-dev/pkg/util"
)

// TokenService menyimpan & membaca access token GitHub (terenkripsi).
type TokenService struct {
	repo   *repository.UserTokenRepository
	secret string
}

func NewTokenService(repo *repository.UserTokenRepository, secret string) *TokenService {
	return &TokenService{repo: repo, secret: secret}
}

// Save mengenkripsi & menyimpan token milik user.
func (s *TokenService) Save(ctx context.Context, u *GitHubUser, tok *GitHubTokenResponse) error {
	enc, err := util.Encrypt(s.secret, tok.AccessToken)
	if err != nil {
		return err
	}
	var encRefresh string
	if tok.RefreshToken != "" {
		encRefresh, err = util.Encrypt(s.secret, tok.RefreshToken)
		if err != nil {
			return err
		}
	}

	now := time.Now()
	var tokenExp *time.Time
	if tok.ExpiresIn > 0 {
		t := now.Add(time.Duration(tok.ExpiresIn) * time.Second)
		tokenExp = &t
	}
	var refreshExp *time.Time
	if tok.RefreshTokenExp > 0 {
		t := now.Add(time.Duration(tok.RefreshTokenExp) * time.Second)
		refreshExp = &t
	}
	return s.repo.Upsert(ctx, &repository.UserToken{
		GitHubID:              u.ID,
		Login:                 u.Login,
		AvatarURL:             u.AvatarURL,
		Name:                  u.Name,
		AccessToken:           enc,
		TokenType:             tok.TokenType,
		Scope:                 tok.Scope,
		RefreshToken:          encRefresh,
		TokenExpiresAt:        tokenExp,
		RefreshTokenExpiresAt: refreshExp,
	})
}

// Get mengembalikan token plaintext milik login; "" jika belum ada.
func (s *TokenService) Get(ctx context.Context, login string) (string, error) {
	t, err := s.repo.FindByLogin(ctx, login)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", nil
	}
	return util.Decrypt(s.secret, t.AccessToken)
}

// SaveRefreshed menyimpan hasil refresh GitHub tanpa menghapus data profil.
func (s *TokenService) SaveRefreshed(ctx context.Context, login string, tok *GitHubTokenResponse) error {
	encAccess, err := util.Encrypt(s.secret, tok.AccessToken)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"access_token": encAccess,
		"token_type":   tok.TokenType,
		"scope":        tok.Scope,
		"updated_at":   time.Now(),
	}
	if tok.RefreshToken != "" {
		encRefresh, err := util.Encrypt(s.secret, tok.RefreshToken)
		if err != nil {
			return err
		}
		updates["refresh_token"] = encRefresh
	}
	if tok.ExpiresIn > 0 {
		updates["token_expires_at"] = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	if tok.RefreshTokenExp > 0 {
		updates["refresh_token_expires_at"] = time.Now().Add(time.Duration(tok.RefreshTokenExp) * time.Second)
	}
	return s.repo.Update(ctx, login, updates)
}

// Profile mengembalikan profil tersimpan (tanpa token).
func (s *TokenService) Profile(ctx context.Context, login string) (*repository.UserToken, error) {
	t, err := s.repo.FindByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errors.New("user tidak ditemukan")
	}
	return t, nil
}

// Revoke menghapus token tersimpan (logout penuh).
func (s *TokenService) Revoke(ctx context.Context, login string) error {
	return s.repo.Delete(ctx, login)
}

// GetRefresh returns the plaintext refresh token; "" if none.
func (s *TokenService) GetRefresh(ctx context.Context, login string) (string, error) {
	t, err := s.repo.FindByLogin(ctx, login)
	if err != nil || t == nil || t.RefreshToken == "" {
		return "", err
	}
	return util.Decrypt(s.secret, t.RefreshToken)
}
