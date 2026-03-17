package loanproduct

import "strings"

func countActiveCoreLoans(loans []CoreCustomerLoanItem) int {
	count := 0
	for _, loan := range loans {
		if isActiveCoreLoanStatus(loan.Status) {
			count++
		}
	}

	return count
}

func exceedsMaxActiveLoans(activeLoanCount, maxActiveLoans int) bool {
	if maxActiveLoans <= 0 {
		return activeLoanCount > 0
	}

	return activeLoanCount >= maxActiveLoans
}

func shouldInspectLoanForOutstandingDefault(loan CoreCustomerLoanItem) bool {
	return loan.OutstandingBalance > 0 || isOutstandingDefaultStatus(loan.Status)
}

func hasOutstandingDefaultLoan(loan *CoreLoanDetail) bool {
	if loan == nil {
		return false
	}

	if loan.OutstandingBalance <= 0 {
		return false
	}

	return isOutstandingDefaultStatus(loan.Status)
}

func isActiveCoreLoanStatus(status string) bool {
	switch normalizeCoreLoanStatus(status) {
	case "active", "running", "disbursed", "current":
		return true
	default:
		return false
	}
}

func isOutstandingDefaultStatus(status string) bool {
	switch normalizeCoreLoanStatus(status) {
	case "default", "defaulted", "past_due", "pastdue", "overdue":
		return true
	default:
		return false
	}
}

func normalizeCoreLoanStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	status = strings.ReplaceAll(status, "-", "_")
	status = strings.ReplaceAll(status, " ", "_")
	return status
}
