package vas

import "strings"

func ExtractBillingCompanyName(text string) string {
	return strings.Split(text, "_")[0]
}
