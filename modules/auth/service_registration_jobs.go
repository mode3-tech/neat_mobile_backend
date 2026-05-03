package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/wallet"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) GetRegistrationStatus(ctx context.Context, jobID string) (*RegistrationJobResponse, error) {
	if s.repo == nil {
		return nil, errors.New("auth repository not configured")
	}

	job, err := s.repo.GetRegistrationJobByID(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}

	return registrationJobResponse(job), nil
}

func (s *Service) ProcessPendingRegistrationJobs(ctx context.Context, limit int) error {
	if s.repo == nil {
		return errors.New("auth repository not configured")
	}

	jobs, err := s.repo.ClaimPendingRegistrationJobs(ctx, limit)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		s.processRegistrationJob(ctx, job)
	}

	return nil
}

func (s *Service) processRegistrationJob(ctx context.Context, job RegistrationJob) {
	snapshot, err := decodeRegistrationSnapshot(job.SnapshotJSON)
	if err != nil {
		_ = s.repo.MarkRegistrationJobFailed(ctx, job.ID, err.Error())
		return
	}

	walletResp, err := s.resolveWalletResponseForJob(ctx, &job, snapshot)
	if err != nil {
		_ = s.repo.MarkRegistrationJobFailed(ctx, job.ID, err.Error())
		return
	}

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		walletRepo := wallet.NewRepository(txDB)
		deviceRepo := device.NewRepository(txDB)

		user := buildUserFromRegistrationSnapshot(job.MobileUserID, job.InternalWalletID, snapshot)
		createdUser, txErr := authRepo.CreateUser(ctx, user)
		if txErr != nil {
			return txErr
		}

		if txErr = authRepo.LinkBVNRecordToUser(ctx, snapshot.BVN, createdUser.ID); txErr != nil {
			return txErr
		}

		walletRecord, txErr := buildWalletRecordFromSnapshot(job.MobileUserID, job.InternalWalletID, walletResp, snapshot)
		if txErr != nil {
			return txErr
		}

		if txErr = walletRepo.CreateWallet(ctx, walletRecord); txErr != nil {
			return txErr
		}

		deviceReq := device.DeviceBindingRequest{
			DeviceID:    snapshot.Device.DeviceID,
			PublicKey:   snapshot.Device.PublicKey,
			DeviceName:  snapshot.Device.DeviceName,
			DeviceModel: snapshot.Device.DeviceModel,
			OS:          snapshot.Device.OS,
			OSVersion:   snapshot.Device.OSVersion,
			AppVersion:  snapshot.Device.AppVersion,
			IP:          snapshot.IP,
		}
		deviceService := device.NewService(*deviceRepo)
		if txErr = deviceService.BindDevice(ctx, job.MobileUserID, &deviceReq); txErr != nil {
			return txErr
		}

		return authRepo.MarkRegistrationJobCompleted(ctx, job.ID)
	})
	if err != nil {
		_ = s.repo.MarkRegistrationJobFailed(ctx, job.ID, err.Error())
		return
	}

	go s.syncAndUpdateCBACustomer(
		context.Background(),
		job.MobileUserID,
		snapshot.BVN,
		walletResp.Wallet.AccountName,
		walletResp.Wallet.AccountNumber,
		walletResp.Wallet.BankCode,
		walletResp.Wallet.BankName,
	)
}

func (s *Service) resolveWalletResponseForJob(ctx context.Context, job *RegistrationJob, snapshot *registrationJobSnapshot) (*WalletResponse, error) {
	if job == nil {
		return nil, errors.New("registration job is required")
	}
	if snapshot == nil {
		return nil, errors.New("registration snapshot is required")
	}
	if s.walletService == nil {
		return nil, errors.New("wallet service not configured")
	}

	if job.WalletResponseJSON != nil && strings.TrimSpace(*job.WalletResponseJSON) != "" {
		resp := &WalletResponse{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(*job.WalletResponseJSON)), resp); err != nil {
			return nil, err
		}
		if err := normalizeWalletResponse(resp, snapshot); err != nil {
			return nil, err
		}
		return resp, nil
	}

	if job.Attempts > 1 {
		resp, found, err := s.lookupWalletResponseForJob(ctx, job, snapshot)
		if err != nil {
			log.Printf("registration job lookup before generate failed job_id=%s customer_id=%s: %v", job.ID, job.MobileUserID, err)
		} else if found {
			return resp, nil
		}
	}

	walletInfo := &WalletPayload{
		BVN:         snapshot.BVN,
		FirstName:   snapshot.FirstName,
		LastName:    snapshot.LastName,
		DateOfBirth: snapshot.DOB.Format("2006-01-02"),
		PhoneNumber: snapshot.Phone,
		Email:       snapshot.WalletEmail,
		Address:     snapshot.WalletAddress,
		Metadata:    map[string]interface{}{"customer_id": job.MobileUserID},
	}

	resp, err := s.walletService.GenerateWallet(ctx, walletInfo)
	if err != nil {
		lookupResp, found, lookupErr := s.lookupWalletResponseForJob(ctx, job, snapshot)
		if lookupErr != nil {
			log.Printf("registration job lookup after generate failure failed job_id=%s customer_id=%s: %v", job.ID, job.MobileUserID, lookupErr)
		} else if found {
			return lookupResp, nil
		}

		return nil, err
	}
	return s.persistWalletResponseForJob(ctx, job, snapshot, resp)
}

func (s *Service) lookupWalletResponseForJob(ctx context.Context, job *RegistrationJob, snapshot *registrationJobSnapshot) (*WalletResponse, bool, error) {
	if job == nil {
		return nil, false, errors.New("registration job is required")
	}
	if snapshot == nil {
		return nil, false, errors.New("registration snapshot is required")
	}
	if s.walletService == nil {
		return nil, false, errors.New("wallet service not configured")
	}

	resp, found, err := s.walletService.LookupWalletByCustomerID(ctx, job.MobileUserID)
	if err != nil || !found {
		return nil, found, err
	}

	persistedResp, err := s.persistWalletResponseForJob(ctx, job, snapshot, resp)
	if err != nil {
		return nil, false, err
	}

	return persistedResp, true, nil
}

func (s *Service) persistWalletResponseForJob(ctx context.Context, job *RegistrationJob, snapshot *registrationJobSnapshot, resp *WalletResponse) (*WalletResponse, error) {
	if job == nil {
		return nil, errors.New("registration job is required")
	}
	if snapshot == nil {
		return nil, errors.New("registration snapshot is required")
	}
	if resp == nil {
		return nil, errors.New("wallet response is required")
	}

	if err := normalizeWalletResponse(resp, snapshot); err != nil {
		return nil, err
	}

	body, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveRegistrationJobWalletResponse(ctx, job.ID, string(body)); err != nil {
		return nil, err
	}

	walletJSON := string(body)
	job.WalletResponseJSON = &walletJSON

	return resp, nil
}

func decodeRegistrationSnapshot(snapshotJSON string) (*registrationJobSnapshot, error) {
	trimmed := strings.TrimSpace(snapshotJSON)
	if trimmed == "" {
		return nil, errors.New("registration snapshot is empty")
	}

	var snapshot registrationJobSnapshot
	if err := json.Unmarshal([]byte(trimmed), &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func normalizeWalletResponse(resp *WalletResponse, snapshot *registrationJobSnapshot) error {
	if resp == nil || resp.Customer == nil || resp.Wallet == nil {
		return errors.New("wallet provider returned incomplete response")
	}
	if snapshot == nil {
		return errors.New("registration snapshot is required")
	}

	if strings.TrimSpace(resp.Customer.ID) == "" || strings.TrimSpace(resp.Wallet.WalletId) == "" || strings.TrimSpace(resp.Wallet.AccountNumber) == "" {
		return errors.New("wallet provider returned incomplete response")
	}

	if resp.Customer.Address == nil || strings.TrimSpace(*resp.Customer.Address) == "" {
		address := snapshot.WalletAddress
		resp.Customer.Address = &address
	}
	if strings.TrimSpace(resp.Customer.Email) == "" {
		resp.Customer.Email = snapshot.WalletEmail
	}
	if strings.TrimSpace(resp.Customer.PhoneNumber) == "" {
		resp.Customer.PhoneNumber = snapshot.Phone
	}
	if strings.TrimSpace(resp.Customer.BVN) == "" {
		resp.Customer.BVN = snapshot.BVN
	}
	if strings.TrimSpace(resp.Customer.FirstName) == "" {
		resp.Customer.FirstName = snapshot.FirstName
	}
	if strings.TrimSpace(resp.Customer.LastName) == "" {
		resp.Customer.LastName = snapshot.LastName
	}
	if strings.TrimSpace(resp.Customer.DateOfBirth) == "" {
		resp.Customer.DateOfBirth = snapshot.DOB.Format("2006-01-02")
	}

	return nil
}

func buildUserFromRegistrationSnapshot(mobileUserID, internalWalletID string, snapshot *registrationJobSnapshot) *models.User {
	var emailPtr *string
	if trimmedEmail := strings.TrimSpace(snapshot.Email); trimmedEmail != "" {
		emailCopy := trimmedEmail
		emailPtr = &emailCopy
	}

	var middleNamePtr *string
	if trimmedMiddleName := strings.TrimSpace(snapshot.MiddleName); trimmedMiddleName != "" {
		middleNameCopy := trimmedMiddleName
		middleNamePtr = &middleNameCopy
	}

	return &models.User{
		ID:                     mobileUserID,
		WalletID:               internalWalletID,
		Phone:                  snapshot.Phone,
		Email:                  emailPtr,
		FirstName:              snapshot.FirstName,
		LastName:               snapshot.LastName,
		MiddleName:             middleNamePtr,
		PasswordHash:           snapshot.PasswordHash,
		PinHash:                snapshot.PinHash,
		IsEmailVerified:        snapshot.IsEmailVerified,
		BVN:                    snapshot.BVN,
		NIN:                    snapshot.NIN,
		DOB:                    snapshot.DOB,
		IsPhoneVerified:        snapshot.IsPhoneVerified,
		IsBvnVerified:          snapshot.IsBvnVerified,
		IsNinVerified:          snapshot.IsNinVerified,
		IsBiometricsEnabled:    snapshot.IsBiometricsEnabled,
		IsNotificationsEnabled: true,
	}
}

func buildWalletRecordFromSnapshot(mobileUserID, internalWalletID string, walletResp *WalletResponse, snapshot *registrationJobSnapshot) (*wallet.CustomerWallet, error) {
	if err := normalizeWalletResponse(walletResp, snapshot); err != nil {
		return nil, err
	}

	return &wallet.CustomerWallet{
		ID:               uuid.NewString(),
		InternalWalletID: internalWalletID,
		MobileUserID:     mobileUserID,
		PhoneNumber:      walletResp.Customer.PhoneNumber,
		WalletCustomerID: walletResp.Customer.ID,
		Metadata:         walletResp.Customer.Metadata,
		BVN:              walletResp.Customer.BVN,
		Currency:         walletResp.Customer.Currency,
		DateOfBirth:      walletResp.Customer.DateOfBirth,
		FirstName:        walletResp.Customer.FirstName,
		LastName:         walletResp.Customer.LastName,
		Email:            walletResp.Customer.Email,
		Address:          *walletResp.Customer.Address,
		MerchantID:       walletResp.Customer.MerchantId,
		Tier:             walletResp.Customer.Tier,
		WalletID:         walletResp.Wallet.WalletId,
		Mode:             walletResp.Customer.Mode,
		BankName:         walletResp.Wallet.BankName,
		BankCode:         walletResp.Wallet.BankCode,
		AccountNumber:    walletResp.Wallet.AccountNumber,
		AccountName:      walletResp.Wallet.AccountName,
		AccountRef:       walletResp.Wallet.AccountReference,
		BookedBalance:    walletResp.Wallet.BookedBalance,
		AvailableBalance: walletResp.Wallet.AvailableBalance,
		Status:           walletResp.Wallet.Status,
		WalletType:       walletResp.Wallet.WalletType,
		Updated:          walletResp.Wallet.Updated,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        nil,
	}, nil
}

func registrationJobResponse(job *RegistrationJob) *RegistrationJobResponse {
	if job == nil {
		return nil
	}

	now := time.Now().UTC()
	canClaimSession := registrationJobCanClaimAt(job, now)

	resp := &RegistrationJobResponse{
		JobID:              job.ID,
		RegistrationStatus: string(job.Status),
		CanLogin:           false,
		CanClaimSession:    canClaimSession,
	}
	if job.SessionClaimedAt == nil && job.SessionClaimExpiresAt != nil {
		resp.ClaimExpiresAt = job.SessionClaimExpiresAt
	}

	return resp
}

func (s *Service) kickRegistrationProcessing() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := s.ProcessPendingRegistrationJobs(ctx, 1); err != nil {
			log.Printf("registration job dispatch: %v", err)
		}
	}()
}
