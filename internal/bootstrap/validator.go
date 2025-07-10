package bootstrap

import (
	"github.com/zenstats/zenstats/pkg/validator"
)

func InitValidator() {
	validator.SetupValidator()
}
