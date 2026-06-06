package api

import (
	"fmt"
	"net/url"
)

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Assignees   []User `json:"assignees"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ListTasksResponse struct {
	TasksByStatus   map[string][]Task `json:"tasksByStatus"`
	HasMoreByStatus map[string]bool   `json:"hasMoreByStatus"`
}

func ListTasks(workspaceName, boardName string, page, limit int) (*ListTasksResponse, error) {
	endpoint := fmt.Sprintf("/tasks?workspace=%s&board=%s&page=%d&limit=%d",
		url.QueryEscape(workspaceName), url.QueryEscape(boardName), page, limit)
	var resp ListTasksResponse
	err := doRequest("GET", endpoint, nil, &resp)
	return &resp, err
}

type AddTaskBody struct {
	BoardID string `json:"boardId"`
	Title   string `json:"title"`
}

func AddTask(boardID, title string) error {
	body := AddTaskBody{BoardID: boardID, Title: title}
	return doRequest("POST", "/tasks", body, nil)
}

type UpdateTaskStatusBody struct {
	Status string `json:"status"`
}

func UpdateTaskStatus(taskID, status string) error {
	endpoint := fmt.Sprintf("/tasks/%s/status", taskID)
	body := UpdateTaskStatusBody{Status: status}
	return doRequest("PUT", endpoint, body, nil)
}

func DeleteTask(taskID string) error {
	endpoint := fmt.Sprintf("/tasks/%s", taskID)
	return doRequest("DELETE", endpoint, nil, nil)
}

func GetTask(taskID string) (*Task, error) {
	endpoint := fmt.Sprintf("/tasks/%s", taskID)
	var task Task
	err := doRequest("GET", endpoint, nil, &task)
	return &task, err
}
