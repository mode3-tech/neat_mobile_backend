package auth

import (
	"context"
	"database/sql"
	"xpress/models"
)

type Repository struct {
	db *sql.DB
}

func NewRespository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email
		FROM wallet_users
		WHERE email = $1
	`

	row := r.db.QueryRowContext(ctx, query, email)

	var u models.User
	if err := row.Scan(&u.ID, &u.Email); err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
		SELECT id, email
		FROM wallet_users
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, userID)

	var u models.User
	if err := row.Scan(&u.ID, &u.Email); err != nil {
		return nil, err
	}

	return &u, nil
}
