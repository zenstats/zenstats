// Package bcrypt 提供 bcrypt 密码哈希与校验功能。
package bcrypt

import "golang.org/x/crypto/bcrypt"

// Generate 生成密码的 bcrypt 哈希（cost=14）。
func Generate(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// Check 验证密码与 bcrypt 哈希是否匹配。
func Check(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
