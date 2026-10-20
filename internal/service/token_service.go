package service

import (
	"context"
	"errors"

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
	return s.repo.Upsert(ctx, &repository.UserToken{
		GitHubID:    u.ID,
		Login:       u.Login,
		AvatarURL:   u.AvatarURL,
		Name:        u.Name,
		AccessToken: enc,
		TokenType:   tok.TokenType,
		Scope:       tok.Scope,
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
