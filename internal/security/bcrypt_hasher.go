package security

type BcryptHasher struct{}

func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{}
}

func (h *BcryptHasher) Hash(password string) (string, error) {
	return HashPassword(password)
}

func (h *BcryptHasher) Compare(plain, hashed string) bool {
	return CheckPassword(plain, hashed)
}
