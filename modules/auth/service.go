package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSigner JWTSigner
}

func NewService(repo *Repository, signer JWTSigner) *Service {
	return &Service{repo: repo, jwtSigner: signer}
}

func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)

	if err != nil {
		return "", errors.New("no account exists with this email")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {
		return "", errors.New("incorrect password")
	}

	accessToken, err := s.jwtSigner.Generate(user.ID, "access")
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken, accessToken string) error {
	isValidAccessToken := s.jwtSigner.ValidAccessToken(accessToken)
	isValidRefreshToken := s.jwtSigner.ValidRefreshToken(refreshToken)
	if !isValidAccessToken || !isValidRefreshToken {
		return errors.New("invalid access or refresh token")
	}
	return nil
}
