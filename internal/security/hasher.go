package security

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}

func CheckPassword(plain, hashed string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
