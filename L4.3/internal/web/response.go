package web

type ResultResponse struct {
	Result any `json:"result"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
