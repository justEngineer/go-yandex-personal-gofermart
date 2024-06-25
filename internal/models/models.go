package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ContextKey string

const UserInfoKey ContextKey = "UserInfoKey"

type TimeRFC3339 struct {
	time.Time
}

func (t *TimeRFC3339) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format(time.RFC3339))
}

func (t *TimeRFC3339) UnmarshalJSON(data []byte) error {
	var buffer string
	if err := json.Unmarshal(data, &buffer); err != nil {
		return err
	}
	res, err := time.Parse(time.RFC3339, buffer)
	if err != nil {
		return err
	}
	t.Time = res
	return nil
}

type UserAuthData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type UserInfo struct {
	ID    string
	Login string
	Hash  string
}

type UserBalance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawalRequest struct {
	ID  *string  `json:"order"`
	Sum *float64 `json:"sum"`
}

type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
}

const (
	StatusNew        string = "NEW"
	StatusProcessing string = "PROCESSING"
	StatusInvalid    string = "INVALID"
	StatusProcessed  string = "PROCESSED"
)

type Order struct {
	ID         string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type OrderInfo struct {
	ID         string
	Status     string
	Accrual    sql.NullFloat64
	UploadedAt time.Time
}

type AccrualInfo struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}
