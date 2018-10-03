package v1alpha1

import (
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
)

type AuthManagerType string

const (
	AuthManagerTypeVault AuthManagerType = "Vault"
)

type AuthManager struct {
	Type       AuthManagerType      `json:"type"`
	ManagerRef *appcat.AppReference `json:"managerRef"`
}

// Store specifies where to store credentials
type Store struct {
	// Specifies the name of the secret
	Secret string `json:"secret"`
}

// LeaseData contains lease info
type LeaseData struct {
	// lease id
	ID string `json:"id,omitempty"`

	// lease duration in seconds
	Duration int64 `json:"duration,omitempty"`

	// lease renew deadline in Unix time
	RenewDeadline int64 `json:"renewDeadline,omitempty"`
}
