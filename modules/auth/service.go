package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"neat_mobile_app_backend/models"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSigner JWTSigner
}

func NewService(repo *Repository, signer JWTSigner) *Service {
	return &Service{repo: repo, jwtSigner: signer}
}

func (s *Service) Login(ctx context.Context, deviceID, ip, userAgent, email, password string) (*AuthObject, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)

	if err != nil {
		fmt.Println("no account exists with this email")
		return nil, errors.New("no account exists with this email")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {
		fmt.Println("incorrect password")
		return nil, errors.New("incorrect password")
	}

	sid := uuid.New().String()

	accessToken, err := s.jwtSigner.IssueAccessToken(user.ID, sid)
	if err != nil {
		fmt.Println("error while issuing access token")
		return nil, err
	}

	authSession := &models.AuthSession{
		UserID:    user.ID,
		SID:       sid,
		DeviceID:  &deviceID,
		IP:        &ip,
		UserAgent: &userAgent,
	}
	refreshToken, jti, _, err := s.jwtSigner.IssueRefreshToken(user.ID, sid)
	if err != nil {
		fmt.Println("error while issuing refresh token")
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))
	if hashedRefreshToken == [32]byte{} {
		fmt.Println("error while hashing refresh token")
		return nil, errors.New("error while hashing refresh token")
	}

	refreshTokenObj := &models.RefreshToken{
		JTI:       jti,
		SessionID: sid,
		UserID:    user.ID,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 30),
	}

	if err := s.repo.AddAccessToken(ctx, authSession); err != nil {
		fmt.Println("error while adding access token")
		return nil, err
	}

	if err := s.repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		fmt.Println("error while adding refresh token")
		return nil, err
	}

	return &AuthObject{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken, accessToken string) error {
	isValidAccessToken := s.jwtSigner.ValidAccessToken(accessToken)
	isValidRefreshToken := s.jwtSigner.ValidRefreshToken(refreshToken)
	if !isValidAccessToken || !isValidRefreshToken {
		return errors.New("invalid access or refresh token")
	}

	accessTokenSub, accessTokenSID, err := s.jwtSigner.ExtractAccessTokenIdentifiers(accessToken)
	if err != nil {
		return err
	}

	refreshTokenSub, refreshTokenSID, jti, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)
	if err != nil {
		return err
	}

	if accessTokenSub != refreshTokenSub {
		return errors.New("access token and refresh token do not match")
	}

	if accessTokenSID != refreshTokenSID {
		return errors.New("access token and refresh token do not match")
	}

	if err = s.repo.DeleteAccessToken(ctx, accessTokenSID); err != nil {
		return err
	}

	if err := s.repo.DeleteRefreshToken(ctx, jti); err != nil {
		return err
	}

	return nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (*AuthObject, error) {
	sub, sid, oldJTI, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)

	if err != nil {
		return nil, errors.New("invalid access token")
	}

	refreshTokenObj, err := s.repo.GetRefreshTokenWithJTI(ctx, oldJTI)

	if err != nil {
		return nil, errors.New("invalid access token")
	}

	now := time.Now().UTC()

	if refreshTokenObj.RevokedAt != nil || refreshTokenObj.SessionID != sid || refreshTokenObj.UserID != sub || refreshTokenObj.ExpiresAt.Before(now) {
		return nil, errors.New("invalid refresh token")
	}

	accessToken, err := s.jwtSigner.IssueAccessToken(sub, sid)
	if err != nil {
		return nil, err
	}

	newRefreshToken, newJTI, newExpiresAt, err := s.jwtSigner.IssueRefreshToken(sub, sid)
	if err != nil || newRefreshToken == "" || newJTI == "" {
		return nil, errors.New("failed to issue refresh token")
	}

	hashedRefreshToken := sha256.Sum256([]byte(newRefreshToken))
	if hashedRefreshToken == [32]byte{} {
		return nil, errors.New("error hashing new refresh token")
	}

	newRefreshTokenRow := &models.RefreshToken{
		JTI:           newJTI,
		SessionID:     sid,
		UserID:        sub,
		TokenHash:     hex.EncodeToString(hashedRefreshToken[:]),
		ReplacedByJTI: &newJTI,
		IssuedAt:      now,
		ExpiresAt:     newExpiresAt,
	}

	if err := s.repo.RotateRefreshToken(ctx, oldJTI, newRefreshTokenRow); err != nil {
		return nil, err
	}

	return &AuthObject{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil

}
