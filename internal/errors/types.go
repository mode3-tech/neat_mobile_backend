package errors

type XpressWalletProviderError struct {
	Status  int
	Code    string
	Message string
}

func (e XpressWalletProviderError) Error() string {
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
