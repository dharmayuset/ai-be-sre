// Package models berisi tipe data domain yang dipakai di banyak layer.
package models

import "time"

// User adalah representasi user FreeIPA dalam domain app.
// Dipakai di handler/response (jangan tambah field sensitif seperti password).
type User struct {
	Username    string    `json:"username"`
	DN          string    `json:"-"` // distinguished name (internal only)
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName"`
	Groups      []string  `json:"groups"`
	IsAdmin     bool      `json:"isAdmin"`
	Locked      bool      `json:"locked"`
	LastLogin   time.Time `json:"lastLogin,omitempty"`
}

// Role yang dikenal aplikasi (bukan disimpan, derived dari grup LDAP).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

func (u *User) Role() Role {
	if u.IsAdmin {
		return RoleAdmin
	}
	return RoleUser
}
