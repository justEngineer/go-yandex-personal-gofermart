package security

import (
	"errors"
	"fmt"

	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func GetHashedPassword(password *string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func VerifyPassword(user *models.UserAuthData, userInfo *models.UserInfo) error {
	if err := bcrypt.CompareHashAndPassword([]byte(userInfo.Hash), []byte(user.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return fmt.Errorf("password is incorrect: %s", user.Password)
		}
		return err
	}
	return nil
}
