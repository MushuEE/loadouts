package core

import "time"

// User is the root identity on the platform. A User is mapped 1:many with Profiles.
// Nothing is authored directly by a User; authorship always hangs off a Profile.
type User struct {
	ID          string    `json:"id" db:"id"`
	Email       string    `json:"email" db:"email"`
	DisplayName string    `json:"display_name" db:"display_name"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Profile is a distinct account owned by a User. Generally interchangeable with a User,
// but a User may keep several Profiles to separate distinct hobbies or personas
// (e.g. "gearhead" for backpacking and "fitcheck" for streetwear).
type Profile struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Handle      string    `json:"handle" db:"handle"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Bio         string    `json:"bio" db:"bio"`
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`
	IsSponsor   bool      `json:"is_sponsor" db:"is_sponsor"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
