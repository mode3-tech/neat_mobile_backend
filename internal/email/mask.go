package email

func MaskEmail(email string) string {
	if len(email) == 0 {
		return ""
	}
	return email[0:1] + "*****" + email[len(email)-1:]
}
