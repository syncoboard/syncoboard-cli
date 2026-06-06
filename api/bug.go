package api

type ReportBugBody struct {
	Message string `json:"message"`
	Browser string `json:"browser"`
}

func ReportBug(message string) error {
	body := ReportBugBody{Message: message, Browser: "TUI CLI"}
	return doRequest("POST", "/bugs", body, nil)
}
