package connect

import (
	"encoding/json"
	"fmt"
)

type FacebookConnectFailureCode string

const (
	FacebookPageNotAvailable       FacebookConnectFailureCode = "facebook_page_not_available"
	FacebookPagePermissionRequired FacebookConnectFailureCode = "facebook_page_permission_required"
	FacebookAuthorizationFailed    FacebookConnectFailureCode = "facebook_authorization_failed"
)

type FacebookConnectFailure struct {
	Code                 FacebookConnectFailureCode
	Stage                string
	RemoteStatusCode     int
	MetaCode             int
	MetaSubcode          int
	PageCount            int
	PublishablePageCount int
}

func (e *FacebookConnectFailure) Error() string {
	if e == nil {
		return "facebook connect failed"
	}
	return fmt.Sprintf("facebook connect failed at %s", e.Stage)
}

func newFacebookFailure(stage string) *FacebookConnectFailure {
	return &FacebookConnectFailure{
		Code:  FacebookAuthorizationFailed,
		Stage: stage,
	}
}

func newFacebookProviderFailure(stage string, status int, body []byte) *FacebookConnectFailure {
	var envelope struct {
		Error struct {
			Code    int `json:"code"`
			Subcode int `json:"error_subcode"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	return &FacebookConnectFailure{
		Code:             FacebookAuthorizationFailed,
		Stage:            stage,
		RemoteStatusCode: status,
		MetaCode:         envelope.Error.Code,
		MetaSubcode:      envelope.Error.Subcode,
	}
}
