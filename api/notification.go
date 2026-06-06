package api

import "time"

type NotificationLog struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
	Actor     *Actor    `json:"actor,omitempty"`
	Board     *Board    `json:"board,omitempty"`
	Task      *TaskLog  `json:"task,omitempty"`
}

type Actor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Board struct {
	Name      string     `json:"name"`
	Workspace *Workspace `json:"workspace,omitempty"`
}

type TaskLog struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

type GetNotificationsResponse struct {
	Logs []NotificationLog `json:"logs"`
}

func GetNotifications() (*GetNotificationsResponse, error) {
	var resp GetNotificationsResponse
	err := doRequest("GET", "/notifications", nil, &resp)
	return &resp, err
}

type GetReadStateResponse struct {
	LastRead string `json:"lastRead"`
}

func GetReadState() (*GetReadStateResponse, error) {
	var resp GetReadStateResponse
	err := doRequest("GET", "/notifications/read-state", nil, &resp)
	return &resp, err
}

func MarkAsRead() error {
	return doRequest("POST", "/notifications/read", nil, nil)
}
