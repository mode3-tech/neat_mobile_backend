package auth

import (
	"context"
	"log"
	"neat_mobile_app_backend/internal"
	"neat_mobile_app_backend/modules/loanproduct"
	"time"
)

func (s *Service) syncAndUpdateCBACustomer(ctx context.Context, userID, bvn, accountName, accountNumber, bankCode, bank string) {
	customerID := s.syncUpCustomerExistingOnCBA(ctx, userID, bvn)
	if customerID == nil {
		return
	}

	s.updateCustomerWalletInfoOnTheCBA(ctx, userID, *customerID, &internal.CustomerUpdateRequest{
		AccountNumber: accountNumber,
		AccountName:   accountName,
		Bank:          bank,
		BankCode:      bankCode,
	})
}

func (s *Service) SyncPendingCBACustomers(ctx context.Context) error {
	users, err := s.repo.GetUsersWithoutCoreCustomerID(ctx, 50)
	if err != nil {
		return err
	}

	if len(users) <= 0 {
		return nil
	}

	for _, u := range users {
		go s.syncAndUpdateCBACustomer(ctx, u.ID, u.BVN, u.AccountName, u.AccountNumber, u.BankCode, u.Bank)
	}

	return nil
}

func (s *Service) syncUpCustomerExistingOnCBA(ctx context.Context, userID, BVN string) *string {
	select {
	case s.cbaSyncSem <- struct{}{}:
		defer func() {
			<-s.cbaSyncSem
		}()
	case <-ctx.Done():
		return nil

	}

	delays := []time.Duration{0, 5 * time.Second, 30 * time.Second}

	for attempt, delay := range delays {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil
			}
		}

		match, err := s.coreCustomerFinder.MatchCustomerByBVN(ctx, BVN)
		if err != nil {
			log.Printf("syncUpCustomerExistingONCBA: attempt %d failed for user %s: %v", attempt+1, userID, err)
			continue
		}

		if match.MatchStatus != loanproduct.CoreCustomerSingleMatch || match.Customer == nil {
			return nil
		}

		if err := s.repo.UpdateCoreCustomerID(ctx, userID, match.Customer.CustomerID); err != nil {
			log.Printf("syncUpCustomerExistingOnCBA: db update failed for user %s: %v", userID, err)
			return nil
		}
		return &match.Customer.CustomerID

	}
	log.Printf("syncUpCustomerExistingOnCBA: all attempts exhausted for user %s", userID)
	return nil
}

func (s *Service) updateCustomerWalletInfoOnTheCBA(ctx context.Context, userID, coreCustomerID string, info *internal.CustomerUpdateRequest) {
	select {
	case s.cbaWalletUpdateSem <- struct{}{}:
		defer func() { <-s.cbaWalletUpdateSem }()
	case <-ctx.Done():
		return
	}

	delays := []time.Duration{0, 5 * time.Second, 30 * time.Second}

	for attempt, delay := range delays {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		_, err := s.cbaCustomerUpdater.UpdateCBACustomerBankInfo(ctx, coreCustomerID, info)
		if err != nil {
			log.Printf("updateCustomerWalletInfoOnTheCBA: attempt %d failed for user %s: %v", attempt+1, userID, err)
			continue
		}
		return
	}

	log.Printf("updateCustomerWalletInfoOnTheCBA: all attempts exhausted for user %s", userID)

}
