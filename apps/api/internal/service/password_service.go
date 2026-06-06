package service

import "golang.org/x/crypto/bcrypt"

// PasswordService hashes and verifies user passwords using bcrypt.
type PasswordService struct {
	hashingCost int
}

func NewPasswordService() *PasswordService {
	return &PasswordService{hashingCost: 12}
}

func (service *PasswordService) HashPassword(plainTextPassword string) (string, error) {
	hashedBytes, hashingError := bcrypt.GenerateFromPassword([]byte(plainTextPassword), service.hashingCost)
	if hashingError != nil {
		return "", hashingError
	}
	return string(hashedBytes), nil
}

func (service *PasswordService) VerifyPassword(passwordHash string, plainTextPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(plainTextPassword)) == nil
}
