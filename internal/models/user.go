package models

type User struct {
	ID       string `json:"id"`
	Login 	 string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UserUpdate struct {
	Login    *string `json:"login,omitempty"`
	Email    *string `json:"email,omitempty"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Login string `json:"username"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}