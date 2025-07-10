.PHONY: swagger
swagger:
	@swag init -g docs/swagger.go --output docs
	@echo "Swagger documentation generated"