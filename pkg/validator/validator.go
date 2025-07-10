package validator

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func SetupValidator() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册自定义验证函数，tag名称为"mobile"
		v.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
			// 简单的手机号验证逻辑
			mobile := fl.Field().String()
			return len(mobile) == 11 && mobile[0] == '1'
		})
	}
}
