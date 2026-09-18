package mail

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed otp.html
var templatesFS embed.FS

type OTPEmailData struct {
	Subject   string
	OTP       string
	Year      int
	ExpiresIn string
}

func RenderOTPEmail(data OTPEmailData) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "otp.html")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
