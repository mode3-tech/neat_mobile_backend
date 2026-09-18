package accountclosure

type AccountClosureRequest struct {
	ReasonNote string `json:"reason_note" binding:"required,max=500"`
}
