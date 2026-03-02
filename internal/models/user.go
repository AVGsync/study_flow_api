package models

type User struct {
	ID       string `json:"id"`
	Login 	 string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UserUpdateRequest struct {
    Login *string `json:"login,omitempty" validate:"omitempty,min=3,max=32,alphanum"`
    Email *string `json:"email,omitempty" validate:"omitempty,email"`
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