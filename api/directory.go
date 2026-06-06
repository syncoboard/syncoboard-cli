package api

import (
	"fmt"
	"net/url"
)

type DirectoryEntry struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
}

type DirectoryResponse struct {
	Path            string           `json:"path"`
	Type            string           `json:"type"`
	ID              string           `json:"id,omitempty"`
	Entries         []DirectoryEntry `json:"entries"`
	HasMoreByStatus map[string]bool  `json:"hasMoreByStatus,omitempty"`
}

func GetDirectory(path string) (*DirectoryResponse, error) {
	var resp DirectoryResponse
	endpoint := fmt.Sprintf("/directory?path=%s", url.QueryEscape(path))
	err := doRequest("GET", endpoint, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
