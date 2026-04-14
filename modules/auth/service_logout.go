package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

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

	if s.deviceRepo != nil {
		session, sessionErr := s.repo.GetAccessTokenWithSID(ctx, accessTokenSID)
		if sessionErr != nil {
			return sessionErr
		}
		if session.DeviceID != nil && strings.TrimSpace(*session.DeviceID) != "" {
			if err = s.deviceRepo.DeactivateDevice(ctx, accessTokenSub, *session.DeviceID); err != nil {
				return err
			}
		}
	}

	if err = s.repo.DeleteAccessToken(ctx, accessTokenSID); err != nil {
		return err
	}

	if err := s.repo.DeleteRefreshToken(ctx, jti); err != nil {
		return err
	}

	return nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, deviceID, refreshToken string) (*AuthObject, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("invalid refresh token")
	}

	sub, sid, oldJTI, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	refreshTokenObj, err := s.repo.GetRefreshTokenWithJTI(ctx, oldJTI)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	if refreshTokenObj.TokenHash != "" {
		receivedHash := sha256.Sum256([]byte(refreshToken))
		if refreshTokenObj.TokenHash != hex.EncodeToString(receivedHash[:]) {
			return nil, errors.New("invalid refresh token")
		}
	}

	if _, err := s.verifyUserDevice(ctx, refreshTokenObj.UserID, deviceID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	if refreshTokenObj.RevokedAt != nil || refreshTokenObj.SessionID != sid || refreshTokenObj.UserID != sub || refreshTokenObj.ExpiresAt.Before(now) {
		return nil, errors.New("invalid refresh token")
	}

	accessSession, err := s.repo.GetAccessTokenWithSID(ctx, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid session")
		}
		return nil, err
	}

	if accessSession.RevokedAt != nil || accessSession.UserID != sub || accessSession.DeviceID == nil || strings.TrimSpace(*accessSession.DeviceID) != deviceID {
		return nil, errors.New("device not allowed")
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
		JTI:       newJTI,
		SessionID: sid,
		UserID:    sub,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  now,
		ExpiresAt: newExpiresAt,
	}

	if err := s.repo.RotateRefreshToken(ctx, oldJTI, newRefreshTokenRow); err != nil {
		return nil, err
	}

	return &AuthObject{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil

}
