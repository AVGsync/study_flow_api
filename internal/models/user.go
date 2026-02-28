package models

type User struct {
	ID       string `json:"id"`
	Login 	 string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}