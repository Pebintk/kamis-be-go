// Package dto holds the profile service's request/response shapes (the Java
// restdto package). Field names match the legacy JSON so the frontend is
// unaffected.
package dto

// LoginRequest is posted to /api/auth/login. Login is by email (legacy behaviour).
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the `data` of a successful login, and of a refresh.
//
// `token` keeps its legacy name and meaning — the access token the frontend
// sends as a bearer — so nothing that already reads response.data.token breaks.
// The rest is new: the access token is now short-lived, and `refreshToken` buys
// the next one.
type LoginResponse struct {
	Token         string `json:"token"`
	RefreshToken  string `json:"refreshToken"`
	ExpiresInSecs int    `json:"expiresInSeconds"`
}

// RefreshRequest is posted to /api/auth/refresh and /api/auth/logout. Both
// authenticate by the refresh token itself, so neither needs a bearer.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// AddUserRequest is posted to /api/profile/add. Role is the lowercase API form.
type AddUserRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// UpdateUserRequest is the body of PUT /api/profile/{id}. Pointers distinguish
// "field omitted" from "set to empty", matching the legacy null checks.
type UpdateUserRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Username *string `json:"username"`
}

// EndUserResponse is the account representation returned to clients. Role is the
// lowercase API form.
type EndUserResponse struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Page is the account listing page. It is an alias so the existing
// EndUserResponse pagination keeps its name while sharing one implementation
// with the client and supplier listings.
type Page = PageOf[EndUserResponse]
