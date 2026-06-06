package api

func UpdateLastOnline() error {
	return doRequest("POST", "/user/activity", nil, nil)
}
