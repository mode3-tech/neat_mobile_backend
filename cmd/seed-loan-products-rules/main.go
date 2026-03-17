package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/modules/loanproduct"
)

type seedRule struct {
	ProductID                   string `json:"product_id"`
	ProductCode                 string `json:"product_code"`
	MinSavingsBalance           int64  `json:"min_savings_balance"`
	MinAccountAgeDays           int    `json:"min_account_age_days"`
	MaxActiveLoans              int    `json:"max_active_loans"`
	LoanTermValue               int    `json:"loan_term_value"`
	RequireKYC                  *bool  `json:"require_kyc"`
	RequireBVN                  *bool  `json:"require_bvn"`
	RequireNIN                  *bool  `json:"require_nin"`
	RequirePhoneVerified        *bool  `json:"require_phone_verified"`
	RequireNoOutstandingDefault *bool  `json:"require_no_outstanding_default"`
	HighValueThreshold          int    `json:"high_value_threshold"`

	// Support existing file key while also allowing the canonical one.
	BranchManagerApproval      *int64 `json:"branch_manager_approval"`
	BranchManagerApprovalLimit *int64 `json:"branch_manager_approval_limit"`

	BranchManagerApprovalResolved int64 `json:"-"`
}

func main() {
	_ = godotenv.Load()

	dir := flag.String("dir", "./seed/loan_product_rules", "directory containing loan product rule json files")
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
		in, err := loadAndValidate(path)
		if err != nil {
			failed++
			log.Printf("skip %s: %v", path, err)
			continue
		}

		productID, err := resolveProductID(db, in, path)
		if err != nil {
			failed++
			log.Printf("skip %s: %v", path, err)
			continue
		}

		row := loanproduct.LoanProductRule{
			ID:                          uuid.NewString(),
			ProductID:                   productID,
			MinSavingsBalance:           in.MinSavingsBalance,
			MinAccountAgeDays:           in.MinAccountAgeDays,
			MaxActiveLoans:              in.MaxActiveLoans,
			RequireKYC:                  in.RequireKYC,
			RequireBVN:                  in.RequireBVN,
			RequireNIN:                  in.RequireNIN,
			RequirePhoneVerified:        in.RequirePhoneVerified,
			RequireNoOutstandingDefault: in.RequireNoOutstandingDefault,
			HighValueThreshold:          in.HighValueThreshold,
			BranchManagerApprovalLimit:  in.BranchManagerApprovalResolved,
		}

		if *dryRun {
			log.Printf("[dry-run] %s -> product_id=%s", filepath.Base(path), row.ProductID)
			continue
		}

		isCreate, err := upsertRuleByProductID(db, row)
		if err != nil {
			failed++
			log.Printf("skip %s: upsert failed: %v", path, err)
			continue
		}

		if isCreate {
			created++
		} else {
			updated++
		}
		log.Printf("seeded rule for product_id=%s (%s)", row.ProductID, filepath.Base(path))
	}

	log.Printf("done: created=%d updated=%d failed=%d", created, updated, failed)
}

func loadAndValidate(path string) (seedRule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return seedRule{}, err
	}

	var in seedRule
	if err := json.Unmarshal(b, &in); err != nil {
		return seedRule{}, err
	}

	in.ProductID = strings.TrimSpace(in.ProductID)
	in.ProductCode = normalizeCode(in.ProductCode)

	if in.ProductID != "" {
		if _, err := uuid.Parse(in.ProductID); err != nil {
			return seedRule{}, errors.New("product_id must be a valid UUID")
		}
	}

	if in.MinSavingsBalance < 0 {
		return seedRule{}, errors.New("min_savings_balance must be >= 0")
	}
	if in.MinAccountAgeDays < 0 {
		return seedRule{}, errors.New("min_account_age_days must be >= 0")
	}
	if in.MaxActiveLoans < 0 {
		return seedRule{}, errors.New("max_active_loans must be >= 0")
	}
	if in.HighValueThreshold < 0 {
		return seedRule{}, errors.New("high_value_threshold must be >= 0")
	}

	approvalLimit := in.BranchManagerApprovalLimit
	if approvalLimit == nil {
		approvalLimit = in.BranchManagerApproval
	}
	if approvalLimit == nil {
		return seedRule{}, errors.New("branch_manager_approval_limit (or branch_manager_approval) is required")
	}
	if *approvalLimit < 0 {
		return seedRule{}, errors.New("branch_manager_approval_limit must be >= 0")
	}
	in.BranchManagerApprovalResolved = *approvalLimit

	return in, nil
}

func resolveProductID(db *gorm.DB, in seedRule, path string) (string, error) {
	if in.ProductID != "" {
		exists, err := productExists(db, in.ProductID)
		if err != nil {
			return "", err
		}
		if exists {
			return in.ProductID, nil
		}
	}

	codesToTry := make([]string, 0, 2)
	if in.ProductCode != "" {
		codesToTry = append(codesToTry, in.ProductCode)
	}
	if inferred := inferProductCode(path); inferred != "" && inferred != in.ProductCode {
		codesToTry = append(codesToTry, inferred)
	}

	for _, code := range codesToTry {
		id, err := productIDByCode(db, code)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}

	if in.ProductID != "" {
		return "", fmt.Errorf("referenced product_id not found: %s (set product_code in json or use a matching filename)", in.ProductID)
	}
	if len(codesToTry) > 0 {
		return "", fmt.Errorf("referenced product code not found: %s", strings.Join(codesToTry, ","))
	}
	return "", errors.New("product_id or product_code is required")
}

func productExists(db *gorm.DB, productID string) (bool, error) {
	var count int64
	err := db.Model(&loanproduct.LoanProduct{}).
		Where("id = ?", productID).
		Count(&count).Error
	return count > 0, err
}

func productIDByCode(db *gorm.DB, code string) (string, error) {
	var p loanproduct.LoanProduct
	err := db.Model(&loanproduct.LoanProduct{}).Where("code = ?", code).First(&p).Error
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func normalizeCode(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToUpper(v)
	v = strings.ReplaceAll(v, "_", "-")
	v = strings.Join(strings.Fields(v), "-")
	return v
}

func inferProductCode(path string) string {
	key := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))))
	switch key {
	case "biz":
		return "BUSINESS-WK"
	case "grp":
		return "GROUP-WK"
	case "ind":
		return "INDIVIDUAL-WK"
	case "sal":
		return "SALARY-MTH"
	case "sme":
		return "SME-WK"
	case "sp":
		return "SPECIAL-WK"
	default:
		return ""
	}
}

func upsertRuleByProductID(db *gorm.DB, row loanproduct.LoanProductRule) (bool, error) {
	var existing loanproduct.LoanProductRule
	err := db.Where("product_id = ?", row.ProductID).First(&existing).Error
	if err == nil {
		updates := loanproduct.LoanProductRule{
			MinSavingsBalance:           row.MinSavingsBalance,
			MinAccountAgeDays:           row.MinAccountAgeDays,
			MaxActiveLoans:              row.MaxActiveLoans,
			RequireKYC:                  row.RequireKYC,
			RequireBVN:                  row.RequireBVN,
			RequireNIN:                  row.RequireNIN,
			RequirePhoneVerified:        row.RequirePhoneVerified,
			RequireNoOutstandingDefault: row.RequireNoOutstandingDefault,
			HighValueThreshold:          row.HighValueThreshold,
			BranchManagerApprovalLimit:  row.BranchManagerApprovalLimit,
		}
		if err := db.Model(&loanproduct.LoanProductRule{}).
			Where("id = ?", existing.ID).
			Select(
				"MinSavingsBalance",
				"MinAccountAgeDays",
				"MaxActiveLoans",
				"RequireKYC",
				"RequireBVN",
				"RequireNIN",
				"RequirePhoneVerified",
				"RequireNoOutstandingDefault",
				"HighValueThreshold",
				"BranchManagerApprovalLimit",
			).
			Updates(updates).Error; err != nil {
			return false, err
		}
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	if err := db.Create(&row).Error; err != nil {
		return false, err
	}
	return true, nil
}
