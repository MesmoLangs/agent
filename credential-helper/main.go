package main

import (
	"bufio"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	githubAPIBase         = "https://api.github.com"
	cachePath             = "/tmp/.github-app-token-cache.json"
	tokenLifetimeSeconds  = 1800
	jwtExpirationSeconds  = 600
	jwtClockDriftSeconds  = 60
	httpTimeoutSeconds    = 30
	cacheFilePermissions  = 0600
)

type tokenCache struct {
	Token     string  `json:"token"`
	CreatedAt float64 `json:"created_at"`
}

type installation struct {
	ID int64 `json:"id"`
}

type accessTokenResponse struct {
	Token string `json:"token"`
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	switch os.Args[1] {
	case "get":
		handleCredentialGet()
	case "token":
		token, err := generateToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "token generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(token)
	case "store", "erase":
		return
	}
}

func handleCredentialGet() {
	info := parseCredentialInput(os.Stdin)
	if info["host"] != "github.com" {
		return
	}

	token, err := generateToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "token generation failed: %v\n", err)
		return
	}

	fmt.Printf("protocol=https\nhost=github.com\nusername=x-access-token\npassword=%s\n\n", token)
}

func parseCredentialInput(r io.Reader) map[string]string {
	info := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			info[k] = v
		}
	}
	return info
}

func generateToken() (string, error) {
	if cached := getCachedToken(); cached != "" {
		return cached, nil
	}

	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		return "", fmt.Errorf("GITHUB_APP_ID not set")
	}

	keyPath := os.Getenv("GITHUB_APP_KEY_PATH")
	if keyPath == "" {
		keyPath = "/home/agent/.github-app-key.pem"
	}

	installationID := os.Getenv("GITHUB_APP_INSTALLATION_ID")

	jwt, err := generateJWT(appID, keyPath)
	if err != nil {
		return "", fmt.Errorf("JWT generation: %w", err)
	}

	token, err := getInstallationToken(jwt, installationID)
	if err != nil {
		return "", fmt.Errorf("installation token: %w", err)
	}

	cacheToken(token)
	return token, nil
}

func generateJWT(appID string, keyPath string) (string, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return "", fmt.Errorf("parse private key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("key is not RSA")
		}
	}

	now := time.Now().Unix()
	header := base64URLEncode(mustJSON(map[string]string{"alg": "RS256", "typ": "JWT"}))
	payload := base64URLEncode(mustJSON(map[string]int64{
		"iat": now - jwtClockDriftSeconds,
		"exp": now + jwtExpirationSeconds,
		"iss": mustParseInt64(appID),
	}))

	signingInput := header + "." + payload
	hash := sha256.Sum256([]byte(signingInput))

	sig, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	return signingInput + "." + base64URLEncode(sig), nil
}

func getInstallationToken(jwt string, installationID string) (string, error) {
	client := &http.Client{Timeout: httpTimeoutSeconds * time.Second}

	if installationID == "" {
		id, err := discoverInstallationID(client, jwt)
		if err != nil {
			return "", err
		}
		installationID = fmt.Sprintf("%d", id)
	}

	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", githubAPIBase, installationID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	setGitHubHeaders(req, jwt)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return tokenResp.Token, nil
}

func discoverInstallationID(client *http.Client, jwt string) (int64, error) {
	url := fmt.Sprintf("%s/app/installations", githubAPIBase)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	setGitHubHeaders(req, jwt)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var installations []installation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	if len(installations) == 0 {
		return 0, fmt.Errorf("no installations found for this GitHub App")
	}

	return installations[0].ID, nil
}

func setGitHubHeaders(req *http.Request, jwt string) {
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "github-app-credential-helper")
}

func getCachedToken() string {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return ""
	}

	var cache tokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ""
	}

	if time.Now().Unix()-int64(cache.CreatedAt) < tokenLifetimeSeconds {
		return cache.Token
	}

	return ""
}

func cacheToken(token string) {
	data, err := json.Marshal(tokenCache{
		Token:     token,
		CreatedAt: float64(time.Now().Unix()),
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(cachePath, data, cacheFilePermissions)
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func mustJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func mustParseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
