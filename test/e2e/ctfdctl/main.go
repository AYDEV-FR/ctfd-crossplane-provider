/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Command ctfdctl initializes a CTFd instance and verifies provider results.
//
// Modes:
//   - setup:     run the CTFd first-boot wizard, print an admin token to stdout.
//   - bootstrap: wait for CTFd, run the wizard (or log in if already set up),
//     mint an admin token and write it into a Kubernetes Secret. Used by the
//     in-cluster bootstrap Job so the whole stack comes up with no manual step.
//   - verify:    assert, through the CTFd API, that the provider created the
//     expected objects.
//
// It talks to CTFd through the same github.com/ctfer-io/go-ctfd client the
// provider uses.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	ctfd "github.com/ctfer-io/go-ctfd/api"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	mode := flag.String("mode", "", "operation: setup, bootstrap or verify")
	url := flag.String("url", envOr("CTFD_URL", "http://localhost:8000"), "CTFd base URL")
	token := flag.String("token", os.Getenv("CTFD_TOKEN"), "CTFd admin API token (verify mode)")
	name := flag.String("admin-name", envOr("CTFD_ADMIN", "admin"), "admin username")
	email := flag.String("admin-email", envOr("CTFD_EMAIL", "admin@example.com"), "admin email")
	password := flag.String("admin-password", envOr("CTFD_PASSWORD", "password"), "admin password")
	secretNS := flag.String("secret-namespace", envOr("SECRET_NAMESPACE", "default"), "namespace of the credentials Secret (bootstrap mode)")
	secretName := flag.String("secret-name", envOr("SECRET_NAME", "ctfd-creds"), "name of the credentials Secret (bootstrap mode)")
	secretKey := flag.String("secret-key", envOr("SECRET_KEY", "credentials"), "key inside the credentials Secret (bootstrap mode)")
	flag.Parse()

	switch *mode {
	case "setup":
		token, err := setupAndToken(*url, *name, *email, *password)
		if err != nil {
			fatalf("setup: %v", err)
		}
		fmt.Println(token)
	case "bootstrap":
		if err := bootstrap(*url, *name, *email, *password, *secretNS, *secretName, *secretKey); err != nil {
			fatalf("bootstrap: %v", err)
		}
		fmt.Fprintln(os.Stderr, "[ctfdctl] bootstrap complete: credentials written to "+*secretNS+"/"+*secretName)
	case "verify":
		if *token == "" {
			fatalf("verify: -token (or CTFD_TOKEN) is required")
		}
		if err := verify(*url, *token); err != nil {
			fatalf("verify: %v", err)
		}
		fmt.Fprintln(os.Stderr, "[ctfdctl] all assertions passed")
	default:
		fatalf("unknown -mode %q (want setup, bootstrap or verify)", *mode)
	}
}

// bootstrap waits for CTFd, ensures it is set up, mints an admin token and
// writes it into a Kubernetes Secret as a `{"url","api_key"}` JSON document.
func bootstrap(url, name, email, password, ns, secretName, secretKey string) error {
	if err := waitReady(url, 5*time.Minute); err != nil {
		return err
	}
	token, err := ensureToken(url, name, email, password)
	if err != nil {
		return err
	}
	creds := fmt.Sprintf(`{"url":%q,"api_key":%q}`, url, token)
	return writeSecret(ns, secretName, secretKey, creds)
}

// waitReady blocks until CTFd answers on /setup (any non-5xx status).
func waitReady(url string, timeout time.Duration) error {
	client := noRedirectClient()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpGet(client, url+"/setup")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("CTFd not ready at %s after %s", url, timeout)
}

// ensureToken returns an admin API token, running the setup wizard on a fresh
// instance or logging in on an already-configured one (idempotent).
func ensureToken(url, name, email, password string) (string, error) {
	if isConfigured(url) {
		return loginAndToken(url, name, password)
	}
	return setupAndToken(url, name, email, password)
}

// isConfigured reports whether CTFd is already set up: a configured instance
// redirects /setup to /, a fresh one serves the wizard (200).
func isConfigured(url string) bool {
	resp, err := httpGet(noRedirectClient(), url+"/setup")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 300 && resp.StatusCode < 400
}

func noRedirectClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func httpGet(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// setupAndToken runs the CTFd first-boot wizard and returns a fresh admin token.
func setupAndToken(url, name, email, password string) (string, error) {
	nonce, session, err := ctfd.GetNonceAndSession(url)
	if err != nil {
		return "", fmt.Errorf("getting initial nonce/session: %w", err)
	}
	client := ctfd.NewClient(url, nonce, session, "")
	if err := client.Setup(&ctfd.SetupParams{
		CTFName:                "ctfd",
		CTFDescription:         "managed by provider-ctfd",
		UserMode:               "users",
		ChallengeVisibility:    "public",
		AccountVisibility:      "public",
		ScoreVisibility:        "public",
		RegistrationVisibility: "public",
		VerifyEmails:           false,
		Name:                   name,
		Email:                  email,
		Password:               password,
		CTFTheme:               "core-beta",
	}); err != nil {
		return "", fmt.Errorf("running setup wizard: %w", err)
	}
	return mintToken(client)
}

// loginAndToken logs in as admin on an already-configured CTFd and returns a
// fresh admin token.
func loginAndToken(url, name, password string) (string, error) {
	nonce, session, err := ctfd.GetNonceAndSession(url)
	if err != nil {
		return "", fmt.Errorf("getting nonce/session: %w", err)
	}
	client := ctfd.NewClient(url, nonce, session, "")
	if err := client.Login(&ctfd.LoginParams{Name: name, Password: password}); err != nil {
		return "", fmt.Errorf("logging in: %w", err)
	}
	return mintToken(client)
}

func mintToken(client *ctfd.Client) (string, error) {
	tok, _, err := client.PostTokens(&ctfd.PostTokensParams{
		Description: "provider-ctfd",
		Expiration:  "2222-02-02",
	})
	if err != nil {
		return "", fmt.Errorf("creating API token: %w", err)
	}
	if tok.Value == nil {
		return "", fmt.Errorf("CTFd returned an empty token (authentication likely failed)")
	}
	return *tok.Value, nil
}

// writeSecret creates or updates the credentials Secret using the in-cluster
// Kubernetes API.
func writeSecret(ns, name, key, value string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("in-cluster config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	ctx := context.Background()
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{key: value},
	}

	_, err = cs.CoreV1().Secrets(ns).Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, gerr := cs.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if gerr != nil {
			return fmt.Errorf("getting existing secret: %w", gerr)
		}
		existing.StringData = map[string]string{key: value}
		if _, uerr := cs.CoreV1().Secrets(ns).Update(ctx, existing, metav1.UpdateOptions{}); uerr != nil {
			return fmt.Errorf("updating secret: %w", uerr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}
	return nil
}

// verify asserts that the resources declared by the example manifests exist.
//
//nolint:gocyclo // a flat sequence of independent assertions; splitting hurts readability
func verify(url, token string) error {
	client := ctfd.NewClient(url, "", "", token)

	chs, err := allChallenges(client)
	if err != nil {
		return err
	}

	welcome, ok := chs["Welcome"]
	if !ok {
		return fmt.Errorf("challenge %q not found", "Welcome")
	}
	if err := expectCounts(client, welcome, 1, 0); err != nil {
		return err
	}

	btl, ok := chs["Break The License 1/2"]
	if !ok {
		return fmt.Errorf("challenge %q not found", "Break The License 1/2")
	}
	if err := expectCounts(client, btl, 2, 2); err != nil {
		return err
	}
	// The example chains the two hints: the 25-point hint requires the
	// 10-point one to be unlocked first.
	if err := expectHintPrereqChain(client, btl); err != nil {
		return err
	}

	pages, _, err := client.GetPages(nil)
	if err != nil {
		return fmt.Errorf("listing pages: %w", err)
	}
	if !anyPage(pages, "rules") {
		return fmt.Errorf("page with route %q not found", "rules")
	}

	cfg, err := configMap(client)
	if err != nil {
		return err
	}
	// These are set by the Settings resource (not by the bootstrap wizard).
	if got := cfg["ctf_name"]; got != "Barbhack CTF" {
		return fmt.Errorf("ctf_name = %q, want %q", got, "Barbhack CTF")
	}
	if got := cfg["user_mode"]; got != "teams" {
		return fmt.Errorf("user_mode = %q, want %q", got, "teams")
	}
	if got := cfg["ctf_theme"]; got != "core-beta" {
		return fmt.Errorf("ctf_theme = %q, want %q", got, "core-beta")
	}
	if cfg["theme_header"] == "" {
		return fmt.Errorf("theme_header is empty, expected the value set by the Settings resource")
	}

	return nil
}

func allChallenges(client *ctfd.Client) (map[string]*ctfd.Challenge, error) {
	view := "admin"
	chs, _, err := client.GetChallenges(&ctfd.GetChallengesParams{View: &view})
	if err != nil {
		return nil, fmt.Errorf("listing challenges: %w", err)
	}
	out := make(map[string]*ctfd.Challenge, len(chs))
	for _, c := range chs {
		out[c.Name] = c
	}
	return out, nil
}

func expectCounts(client *ctfd.Client, ch *ctfd.Challenge, flags, hints int) error {
	fs, _, err := client.GetChallengeFlags(ch.ID)
	if err != nil {
		return fmt.Errorf("listing flags of %q: %w", ch.Name, err)
	}
	if len(fs) != flags {
		return fmt.Errorf("challenge %q has %d flags, want %d", ch.Name, len(fs), flags)
	}
	hs, _, err := client.GetChallengeHints(ch.ID)
	if err != nil {
		return fmt.Errorf("listing hints of %q: %w", ch.Name, err)
	}
	if len(hs) != hints {
		return fmt.Errorf("challenge %q has %d hints, want %d", ch.Name, len(hs), hints)
	}
	return nil
}

// expectHintPrereqChain asserts the provider wired the challenge's hints into
// the prerequisite chain declared in the example: the costlier hint must
// require the cheaper one, and the cheaper one must have no prerequisite. The
// challenge hint listing omits requirements, so each hint is fetched
// individually (with preview, as admin) to read them back.
func expectHintPrereqChain(client *ctfd.Client, ch *ctfd.Challenge) error {
	hints, err := fullHints(client, ch)
	if err != nil {
		return err
	}

	base := hintByCost(hints, 10)  // free/cheaper hint, no prerequisite
	gated := hintByCost(hints, 25) // requires base
	if base == nil || gated == nil {
		return fmt.Errorf("challenge %q: expected hints costing 10 and 25, got %d hints", ch.Name, len(hints))
	}

	if prereqs := hintPrereqs(base); len(prereqs) != 0 {
		return fmt.Errorf("challenge %q: 10-point hint should have no prerequisites, got %v", ch.Name, prereqs)
	}
	if got := hintPrereqs(gated); len(got) != 1 || got[0] != base.ID {
		return fmt.Errorf("challenge %q: 25-point hint prerequisites = %v, want [%d]", ch.Name, got, base.ID)
	}
	return nil
}

// hintByCost returns the first hint with the given cost, or nil.
func hintByCost(hints []*ctfd.Hint, cost int) *ctfd.Hint {
	for _, h := range hints {
		if h.Cost == cost {
			return h
		}
	}
	return nil
}

// hintPrereqs returns a hint's prerequisite IDs, or nil when it has none.
func hintPrereqs(h *ctfd.Hint) []int {
	if h.Requirements == nil {
		return nil
	}
	return h.Requirements.Prerequisites
}

// fullHints lists a challenge's hints and resolves each one individually (with
// preview, as admin), since the challenge hint listing omits requirements.
func fullHints(client *ctfd.Client, ch *ctfd.Challenge) ([]*ctfd.Hint, error) {
	hs, _, err := client.GetChallengeHints(ch.ID)
	if err != nil {
		return nil, fmt.Errorf("listing hints of %q: %w", ch.Name, err)
	}
	preview := true
	out := make([]*ctfd.Hint, 0, len(hs))
	for _, h := range hs {
		full, _, err := client.GetHint(strconv.Itoa(h.ID), &ctfd.GetHintParams{Preview: &preview})
		if err != nil {
			return nil, fmt.Errorf("getting hint %d of %q: %w", h.ID, ch.Name, err)
		}
		out = append(out, full)
	}
	return out, nil
}

func anyPage(pages []*ctfd.Page, route string) bool {
	for _, p := range pages {
		if p.Route == route {
			return true
		}
	}
	return false
}

func configMap(client *ctfd.Client) (map[string]string, error) {
	cfgs, _, err := client.GetConfigs(nil)
	if err != nil {
		return nil, fmt.Errorf("listing configs: %w", err)
	}
	out := make(map[string]string, len(cfgs))
	for _, c := range cfgs {
		out[c.Key] = c.Value
	}
	return out, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ctfdctl] "+format+"\n", args...)
	os.Exit(1)
}
