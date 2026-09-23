package auditlog

import "context"

type AuditLogger interface {
	CreateAuditLog(ctx context.Context, log *AuditLog) error
}
