package auditlog

import "net/http"

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}
func (h *Handler) CreateAuditLog(w http.ResponseWriter, r *http.Request) {
	//
}
