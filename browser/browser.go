package browser

import (
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	binPath string
	cookier cookies.Cookier
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

// WithCookier sets a custom cookie store (e.g. S3-backed) for the browser.
func WithCookier(c cookies.Cookier) Option {
	return func(cfg *browserConfig) {
		cfg.cookier = c
	}
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword("***", "***")
	} else {
		u.User = url.User("***")
	}
	return u.String()
}

func NewBrowser(headless bool, options ...Option) *headless_browser.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	opts := []headless_browser.Option{
		headless_browser.WithHeadless(headless),
	}
	if cfg.binPath != "" {
		opts = append(opts, headless_browser.WithChromeBinPath(cfg.binPath))
	}

	// Read proxy from environment variable
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		opts = append(opts, headless_browser.WithProxy(proxy))
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	// 加载 cookies — use injected cookier if provided, otherwise default
	cookier := cfg.cookier
	if cookier == nil {
		cookier = cookies.NewCookier()
	}

	if data, err := cookier.LoadCookies(); err == nil {
		opts = append(opts, headless_browser.WithCookies(string(data)))
		logrus.Infof("Loaded cookies successfully (%d bytes)", len(data))
	} else {
		logrus.Warnf("Failed to load cookies: %v", err)
	}

	return headless_browser.New(opts...)
}
