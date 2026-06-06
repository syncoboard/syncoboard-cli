package api

import "fmt"

type DeletedBoard struct {
	Name           string `json:"name"`
	WorkspaceName  string `json:"workspaceName"`
	RepositoryName string `json:"repositoryName"`
	TimeLeftString string `json:"timeLeftString"`
}

func GetDeletedBoards() ([]DeletedBoard, error) {
	var boards []DeletedBoard
	err := doRequest("GET", "/boards/deleted", nil, &boards)
	return boards, err
}

func DeleteBoard(workspaceName, boardName string) error {
	endpoint := fmt.Sprintf("/boards?workspace=%s&board=%s", workspaceName, boardName)
	return doRequest("DELETE", endpoint, nil, nil)
}

func RestoreBoard(workspaceName, boardName string) error {
	endpoint := fmt.Sprintf("/boards/restore?workspace=%s&board=%s", workspaceName, boardName)
	return doRequest("PUT", endpoint, nil, nil)
}

type UpdateBoardStatusBody struct {
	WorkspaceName string `json:"workspaceName"`
	BoardName     string `json:"boardName"`
	IsActive      bool   `json:"isActive"`
}

func UpdateBoardStatus(workspaceName, boardName string, isActive bool) error {
	endpoint := "/boards/status"
	body := UpdateBoardStatusBody{
		WorkspaceName: workspaceName,
		BoardName:     boardName,
		IsActive:      isActive,
	}
	return doRequest("PUT", endpoint, body, nil)
}

type InviteMemberBody struct {
	WorkspaceName string `json:"workspaceName"`
	BoardName     string `json:"boardName"`
	Identifier    string `json:"identifier"`
}

func InviteMember(workspaceName, boardName, identifier string) error {
	endpoint := "/boards/members"
	body := InviteMemberBody{
		WorkspaceName: workspaceName,
		BoardName:     boardName,
		Identifier:    identifier,
	}
	return doRequest("POST", endpoint, body, nil)
}

func RemoveMember(workspaceName, boardName, identifier string) error {
	endpoint := fmt.Sprintf("/boards/members?workspace=%s&board=%s&identifier=%s", workspaceName, boardName, identifier)
	return doRequest("DELETE", endpoint, nil, nil)
}
