package validators

import "errors"

func ValidatePassword(pw string) error {
	if len(pw) < 8 {
		return errors.New("password length should be at least 8 characters long")
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool

	for _, r := range pw {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasNumber = true
		case r == '!' || r == '@' || r == '#' || r == '$' || r == '%' || r == '^' || r == '&' || r == '*':
			hasSpecial = true
		}

		if hasUpper && hasLower && hasNumber && hasSpecial {
			return nil
		}
	}

	return errors.New("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
}

func ValidatePin(pin string) error {
	if len(pin) != 4 {
		return errors.New("transaction pin must be exactly 4 digits long")
	}

	for _, r := range pin {
		if r < '0' || r > '9' {
			return errors.New("transaction pin must contain only digits")
		}
	}

	return nil
}
