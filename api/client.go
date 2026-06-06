package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

var (
	BaseURL   = "https://syncoboard.com/api"
	AuthToken string
)

func init() {
	if url := os.Getenv("API_URL"); url != "" {
		BaseURL = url
	}
}

type APIError struct {
	Message string `json:"error"`
}

func (e *APIError) Error() string {
	return e.Message
}

func doRequest(method, endpoint string, body interface{}, out interface{}) error {
	url := fmt.Sprintf("%s%s", BaseURL, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", AuthToken))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return &apiErr
		}
		return fmt.Errorf("API error: status code %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}

	return nil
}
