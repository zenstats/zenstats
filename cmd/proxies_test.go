package cmd

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/pkg/iputil"
)

func TestConfigureTrustedProxies(t *testing.T) {
	old := config.Conf.TrustedProxies
	t.Cleanup(func() { config.Conf.TrustedProxies = old; _ = iputil.SetTrustedProxies(old) })
	for _, trusted := range []bool{false, true} {
		config.Conf.TrustedProxies = []string{}
		want := "127.0.0.1"
		if trusted {
			config.Conf.TrustedProxies = []string{"127.0.0.1"}
			want = "8.8.8.8"
		}
		r := gin.New()
		if err := configureTrustedProxies(r); err != nil {
			t.Fatal(err)
		}
		r.GET("/", func(c *gin.Context) {
			if c.ClientIP() != want || iputil.ClientIP(c.Request) != want {
				t.Errorf("gin=%s iputil=%s want=%s", c.ClientIP(), iputil.ClientIP(c.Request), want)
			}
		})
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:80"
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	config.Conf.TrustedProxies = []string{"invalid"}
	if err := configureTrustedProxies(gin.New()); err == nil {
		t.Fatal("bad proxies accepted")
	}
}
