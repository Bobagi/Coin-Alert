package httpserver

import "testing"

// TestIsAllowedWalletURL locks in the SSRF guard: only plain http(s) URLs on investidor10.com.br
// (or a subdomain) may be handed to the server-side scraper.
func TestIsAllowedWalletURL(t *testing.T) {
	allowed := []string{
		"https://investidor10.com.br/carteiras/1234567/",
		"http://investidor10.com.br/carteiras/1/",
		"https://www.investidor10.com.br/carteiras/1/",
		"https://INVESTIDOR10.com.br/carteiras/1/", // host comparison is case-insensitive
	}
	for _, candidate := range allowed {
		if !isAllowedWalletURL(candidate) {
			t.Errorf("expected %q to be allowed", candidate)
		}
	}

	denied := []string{
		"",
		"ftp://investidor10.com.br/x",            // non-http scheme
		"file:///etc/passwd",                     // local file
		"http://169.254.169.254/latest/meta-data/", // cloud metadata
		"http://localhost:5020/health",           // loopback
		"http://scraper:5000/",                   // internal compose service
		"https://investidor10.com.br.evil.com/x", // look-alike parent domain
		"https://evilinvestidor10.com.br/x",      // look-alike prefix
		"https://example.com/investidor10.com.br", // host is not investidor10
	}
	for _, candidate := range denied {
		if isAllowedWalletURL(candidate) {
			t.Errorf("expected %q to be denied", candidate)
		}
	}
}
