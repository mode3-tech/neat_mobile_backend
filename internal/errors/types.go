package errors

type XpressPayProviderError struct {
	Status  int
	Code    string
	Message string
}

func (e XpressPayProviderError) Error() string {
	return e.Message
}

type TermiiError struct {
	Code    int
	Message string
	Status  string
	Link    string
}

func (e TermiiError) Error() string {
	return e.Message
}

type ZeptoError struct {
	Status    int    // HTTP status from ZeptoMail (or synthetic for network/timeout)
	Code      string // top-level error.code, e.g. "TM_3201"
	SubCode   string // details[0].code, e.g. "GE_102"
	Message   string // human-readable reason
	Target    string // details[0].target — the offending field
	RequestID string // request_id
}

func (e ZeptoError) Error() string {
	return e.Message
}

type XpressWalletProviderError struct {
	Status  bool
	Message string
}

func (e XpressWalletProviderError) Error() string {
	return e.Message
}
