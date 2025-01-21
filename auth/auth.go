package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

func generateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}

func GetSpotifyUserAuth() (string, error) {
	codeChan := make(chan string)
	var server *http.Server
	var wg sync.WaitGroup

	server = &http.Server{Addr: ":8888"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		codeChan <- code

		fmt.Fprintf(w, "Authorization successful! You can close this window.")

		go func() {
			server.Shutdown(context.Background())
		}()
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.ListenAndServe()
	}()

	clientID := "dc1f50f63813449f907e16f94997ebab"
	redirectURI := "http://localhost:8888/callback"
	scope := "playlist-read-private playlist-read-collaborative playlist-modify-public playlist-modify-private"

	authURL := fmt.Sprintf("https://accounts.spotify.com/authorize?"+
		"client_id=%s"+
		"&response_type=code"+
		"&redirect_uri=%s"+
		"&scope=%s",
		clientID, url.QueryEscape(redirectURI), url.QueryEscape(scope))

	fmt.Println("Opening browser for Spotify authorization...")
	fmt.Println("If the browser doesn't open automatically, please visit:", authURL)

	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", authURL).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", authURL).Start()
	case "darwin":
		err = exec.Command("open", authURL).Start()
	}
	if err != nil {
		fmt.Println("Please open the URL manually in your browser:", authURL)
	}

	code := <-codeChan

	token, err := exchangeCodeForToken(code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	wg.Wait()

	return token, nil
}

func exchangeCodeForToken(code string) (string, error) {
	clientID := "dc1f50f63813449f907e16f94997ebab"
	clientSecret := "5b7cb2eef8ca48e291e6f6f5e4f0e661"

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", "http://localhost:8888/callback")
	data.Set("client_id", clientID)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}
