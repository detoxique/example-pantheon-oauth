package pantheon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var oauthHTTP = &http.Client{Timeout: 15 * time.Second}

func authorize(ctx context.Context, cfg Config) (tokens, error) {
	state := randomToken(32)
	verifier := randomToken(48)
	sum := sha256.Sum256([]byte(verifier))

	authURL, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return tokens{}, err
	}
	query := authURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURI)
	query.Set("scope", RequiredScopes)
	query.Set("state", state)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	query.Set("code_challenge_method", "S256")
	authURL.RawQuery = query.Encode()

	result := make(chan tokens, 1)
	failures := make(chan error, 1)
	callbackURL, _ := url.Parse(cfg.RedirectURI)
	mux := http.NewServeMux()
	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	mux.HandleFunc(callbackURL.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "OAuth state does not match", http.StatusBadRequest)
			return
		}
		if oauthError := r.URL.Query().Get("error"); oauthError != "" {
			err := fmt.Errorf("OAuth error: %s", oauthError)
			http.Error(w, err.Error(), http.StatusBadRequest)
			select {
			case failures <- err:
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "OAuth code is missing", http.StatusBadRequest)
			return
		}
		value, err := tokenRequest(r.Context(), cfg, url.Values{
			"grant_type": {"authorization_code"}, "client_id": {cfg.ClientID}, "code": {code},
			"redirect_uri": {cfg.RedirectURI}, "code_verifier": {verifier},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			select {
			case failures <- err:
			default:
			}
			return
		}
		_, _ = io.WriteString(w, "Авторизация завершена. Можно закрыть вкладку и вернуться к боту.")
		select {
		case result <- value:
		default:
		}
	})

	listenerError := make(chan error, 1)
	go func() {
		fmt.Printf("Откройте ссылку в браузере:\n%s\n", authURL.String())
		listenerError <- server.ListenAndServe()
	}()
	defer server.Shutdown(context.Background())

	select {
	case <-ctx.Done():
		return tokens{}, ctx.Err()
	case err := <-listenerError:
		if errors.Is(err, http.ErrServerClosed) {
			return tokens{}, ctx.Err()
		}
		return tokens{}, err
	case err := <-failures:
		return tokens{}, err
	case value := <-result:
		return value, nil
	}
}

func tokenRequest(ctx context.Context, cfg Config, form url.Values) (tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cfg.ClientSecret != "" {
		req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	}
	response, err := oauthHTTP.Do(req)
	if err != nil {
		return tokens{}, err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return tokens{}, responseError(response)
	}
	var value tokens
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return tokens{}, err
	}
	if value.AccessToken == "" || value.ExpiresIn <= 0 {
		return tokens{}, errors.New("token endpoint returned an invalid response")
	}
	value.expiresAt = time.Now().Add(time.Duration(value.ExpiresIn) * time.Second)
	return value, nil
}

func randomToken(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
