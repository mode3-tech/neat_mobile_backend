package helpers

import (
	"fmt"

	"github.com/google/uuid"
)

var resourceInitials = map[string]string{
	"wallet":           "wlt",
	"bvn":              "bvn",
	"nin":              "nin",
	"tier_upgrade":     "tr_upgrd",
	"transaction":      "txn",
	"loan":             "ln",
	"loan_application": "ln_app",
	"device":           "dev",
	"session":          "sesh",
	"audit_log":        "adt_log",
}

// init fails fast at startup if two resources were accidentally given the
// same initials, rather than letting the collision surface later as
// ambiguous ID prefixes in production.
func init() {
	seen := make(map[string]string, len(resourceInitials))
	for resource, initials := range resourceInitials {
		if existing, ok := seen[initials]; ok {
			panic(fmt.Sprintf("resourceInitials: %q and %q both map to %q", existing, resource, initials))
		}
		seen[initials] = resource
	}
}

// PrefixID generates a unique ID for resource: looks up its short prefix in
// resourceInitials and appends a random UUID, joined with a hyphen, e.g.
// PrefixID("tier_upgrade") -> "tu-3fa85f64-5717-4562-...". If resource isn't
// registered in resourceInitials, the prefix is empty ("-<uuid>") rather
// than an error - callers should only pass resource names already listed
// there.
func PrefixID(resource string) string {
	prefix := resourceInitials[resource]
	random := uuid.NewString()
	return fmt.Sprintf("%s-%s", prefix, random)
}
