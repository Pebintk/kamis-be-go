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

// AddUserRequest is posted to /api/profile/add. Roles are the lowercase API form.
type AddUserRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required"`
	// Role is the primary role, required and unchanged.
	Role string `json:"role" binding:"required"`
	// Roles is every other role the account should hold. Omitting it gives an
	// account with just the primary role, which is what every caller sent
	// before multi-role existed.
	Roles []string `json:"roles"`
}

// UpdateUserRequest is the body of PUT /api/profile/{id}. Pointers distinguish
// "field omitted" from "set to empty", matching the legacy null checks.
type UpdateUserRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Username *string `json:"username"`
	// Roles replaces the account's whole role set when present. The first entry
	// becomes the primary role.
	Roles *[]string `json:"roles"`
}

// EndUserResponse is the account representation returned to clients. Role is the
// lowercase API form.
type EndUserResponse struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	// Role is the primary role, kept for callers written before multi-role.
	Role string `json:"role"`
	// Roles is every role the account holds, primary first.
	Roles []string `json:"roles"`
}

// Page is the account listing page. It is an alias so the existing
// EndUserResponse pagination keeps its name while sharing one implementation
// with the client and supplier listings.
type Page = PageOf[EndUserResponse]
