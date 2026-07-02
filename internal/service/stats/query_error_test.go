package stats

import (
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// TestTranslateQueryErrorMalformed 验证非法查询的 CH 异常被翻译成 400 校验错误。
func TestTranslateQueryErrorMalformed(t *testing.T) {
	for _, code := range []int32{47, 62, 46, 215} {
		err := translateQueryError(&clickhouse.Exception{Code: code, Message: "boom"})
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("code %d: expected ValidationError, got %T", code, err)
		}
		if ve.StatusCode != 400 {
			t.Fatalf("code %d: expected 400, got %d", code, ve.StatusCode)
		}
	}
}

// TestTranslateQueryErrorTransient 验证非"查询构造"类错误不被当作不支持（仍按执行失败返回）。
func TestTranslateQueryErrorTransient(t *testing.T) {
	// 连接类/其他错误码不在 malformed 集合里
	err := translateQueryError(&clickhouse.Exception{Code: 999, Message: "conn"})
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("transient CH error should not be a ValidationError, got %v", ve)
	}
	// 普通非 CH 错误
	err = translateQueryError(errors.New("dial tcp: connection refused"))
	if errors.As(err, &ve) {
		t.Fatal("plain error should not be a ValidationError")
	}
}

// TestConvertValueSanitizesNonFinite 验证 NaN/Inf 浮点被归零，避免 JSON 序列化失败导致空响应。
func TestConvertValueSanitizesNonFinite(t *testing.T) {
	if got := convertValue(mathNaN()); got != 0.0 {
		t.Fatalf("NaN should sanitize to 0, got %v", got)
	}
	if got := convertValue(mathInf()); got != 0.0 {
		t.Fatalf("Inf should sanitize to 0, got %v", got)
	}
	if got := convertValue(3.5); got != 3.5 {
		t.Fatalf("finite float must pass through, got %v", got)
	}
}

func mathNaN() float64 { var z float64; return z / z }
func mathInf() float64 { var z float64; return 1 / z }
