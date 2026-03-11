package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/modules/loanproduct"
)

type seedProduct struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	MinLoanAmount         int64  `json:"min_loan_amount"`
	MaxLoanAmount         int64  `json:"max_loan_amount"`
	InterestRateBPS       int    `json:"interest_rate_bps"`
	RepaymentFrequency    string `json:"repayment_frequency"` // weekly|monthly
	GracePeriodDays       int    `json:"grace_period_days"`
	LatePenaltyBPS        int    `json:"late_penalty_bps"`
	AllowsConcurrentLoans bool   `json:"allows_concurrent_loans"`
	IsActive              *bool  `json:"is_active"`
}

var codeRe = regexp.MustCompile(`^[A-Z0-9-]{3,30}$`)

func main() {
	_ = godotenv.Load()

	dir := flag.String("dir", "./seed/loan_products", "directory containing product json files")
	dryRun := flag.Bool("dry-run", false, "validate and print only")
	flag.Parse()

	cfg := config.Load()
	if strings.TrimSpace(cfg.DBUrl) == "" {
		log.Fatal("DB_URL is required")
	}

	db, err := database.NewPostgres(cfg.DBUrl)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(*dir, "*.json"))
	if err != nil {
		log.Fatalf("failed reading directory: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no json files found in %s", *dir)
	}

	var created, updated, failed int
	for _, path := range files {
		row, err := loadAndValidate(path)
		if err != nil {
			failed++
			log.Printf("skip %s: %v", path, err)
			continue
		}

		if *dryRun {
			log.Printf("[dry-run] %s -> code=%s name=%s", filepath.Base(path), row.Code, row.Name)
			continue
		}

		exists, err := existsByCode(db, row.Code)
		if err != nil {
			failed++
			log.Printf("skip %s: exists check failed: %v", path, err)
			continue
		}

		err = db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "code"}},
			DoUpdates: clause.Assignments(map[string]any{
				"name":                    row.Name,
				"description":             row.Description,
				"min_loan_amount":         row.MinLoanAmount,
				"max_loan_amount":         row.MaxLoanAmount,
				"interest_rate_bps":       row.InterestRateBPS,
				"repayment_frequency":     row.RepaymentFrequency,
				"grace_period_days":       row.GracePeriodDays,
				"late_penalty_bps":        row.LatePenaltyBPS,
				"allows_concurrent_loans": row.AllowsConcurrentLoans,
				"is_active":               row.IsActive,
				"updated_at":              time.Now().UTC(),
			}),
		}).Create(&row).Error
		if err != nil {
			failed++
			log.Printf("skip %s: upsert failed: %v", path, err)
			continue
		}

		if exists {
			updated++
		} else {
			created++
		}
		log.Printf("seeded %s (%s)", row.Code, filepath.Base(path))
	}

	log.Printf("done: created=%d updated=%d failed=%d", created, updated, failed)
}

func loadAndValidate(path string) (loanproduct.LoanProduct, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return loanproduct.LoanProduct{}, err
	}

	var in seedProduct
	if err := json.Unmarshal(b, &in); err != nil {
		return loanproduct.LoanProduct{}, err
	}

	code := normalizeCode(in.Code)
	if !codeRe.MatchString(code) {
		return loanproduct.LoanProduct{}, errors.New("invalid code format")
	}
	if strings.TrimSpace(in.Name) == "" {
		return loanproduct.LoanProduct{}, errors.New("name is required")
	}
	if in.MinLoanAmount <= 0 {
		return loanproduct.LoanProduct{}, errors.New("min_loan_amount must be > 0")
	}
	if in.MaxLoanAmount < in.MinLoanAmount {
		return loanproduct.LoanProduct{}, errors.New("max_loan_amount must be >= min_loan_amount")
	}
	if in.InterestRateBPS < 0 {
		return loanproduct.LoanProduct{}, errors.New("interest_rate_bps must be >= 0")
	}
	if in.LatePenaltyBPS < 0 {
		return loanproduct.LoanProduct{}, errors.New("late_penalty_bps must be >= 0")
	}
	if in.GracePeriodDays < 0 {
		return loanproduct.LoanProduct{}, errors.New("grace_period_days must be >= 0")
	}

	freq := strings.ToLower(strings.TrimSpace(in.RepaymentFrequency))
	if freq != "weekly" && freq != "monthly" {
		return loanproduct.LoanProduct{}, fmt.Errorf("invalid repayment_frequency: %s", in.RepaymentFrequency)
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	now := time.Now().UTC()
	return loanproduct.LoanProduct{
		ID:                    uuid.NewString(),
		Code:                  code,
		Name:                  strings.TrimSpace(in.Name),
		Description:           strings.TrimSpace(in.Description),
		MinLoanAmount:         in.MinLoanAmount,
		MaxLoanAmount:         in.MaxLoanAmount,
		InterestRateBPS:       in.InterestRateBPS,
		RepaymentFrequency:    loanproduct.LoanFrequency(freq),
		GracePeriodDays:       in.GracePeriodDays,
		LatePenaltyBPS:        in.LatePenaltyBPS,
		AllowsConcurrentLoans: in.AllowsConcurrentLoans,
		IsActive:              isActive,
		CreatedAt:             now,
	}, nil
}

func normalizeCode(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToUpper(v)
	v = strings.ReplaceAll(v, "_", "-")
	v = strings.Join(strings.Fields(v), "-")
	return v
}

func existsByCode(db *gorm.DB, code string) (bool, error) {
	var count int64
	err := db.Model(&loanproduct.LoanProduct{}).
		Where("code = ?", code).
		Count(&count).Error
	return count > 0, err
}
