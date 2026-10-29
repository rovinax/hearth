package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/config"
	"hearth/internal/media"
	"hearth/internal/server"
	"hearth/internal/store"
)

func TestVisibilityAndAdmin(t *testing.T) {
	ts, st := newApp(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "admin", hash, store.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	admin := newClient()
	adminCSRF := login(t, admin, ts.URL, "admin", "password1")
	createUser(t, admin, ts.URL, adminCSRF, "ada", store.RoleUser)
	createUser(t, admin, ts.URL, adminCSRF, "bob", store.RoleUser)

	ada := newClient()
	adaCSRF := login(t, ada, ts.URL, "ada", "password1")
	upload(t, ada, ts.URL, adaCSRF, "private", "a-private.txt", []byte("secret-ada"))
	upload(t, ada, ts.URL, adaCSRF, "shared", "a-shared.txt", []byte("shared-ada"))
	adaFiles := listFiles(t, ada, ts.URL+"/api/files")
	privateID := fileID(t, adaFiles, "a-private.txt")
	sharedID := fileID(t, adaFiles, "a-shared.txt")

	bob := newClient()
	bobCSRF := login(t, bob, ts.URL, "bob", "password1")
	bobFiles := listFiles(t, bob, ts.URL+"/api/files")
	if _, ok := findFile(bobFiles, "a-private.txt"); ok {
		t.Fatal("bob can see private file")
	}
	if _, ok := findFile(bobFiles, "a-shared.txt"); !ok {
		t.Fatal("bob cannot see shared file")
	}
	if res := getStatus(t, bob, ts.URL+"/api/files/"+privateID+"/content"); res != http.StatusNotFound {
		t.Fatalf("private download: %d", res)
	}
	if res := getStatus(t, bob, ts.URL+"/api/files/"+privateID); res != http.StatusNotFound {
		t.Fatalf("private view: %d", res)
	}
	if body := get(t, bob, ts.URL+"/api/files/"+sharedID+"/content"); body != "shared-ada" {
		t.Fatalf("shared download: %q", body)
	}
	res := sendJSON(t, bob, http.MethodDelete, ts.URL+"/api/files/"+sharedID, bobCSRF, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("bob delete: %d", res.StatusCode)
	}
	res.Body.Close()
	if body := get(t, ada, ts.URL+"/api/files/"+sharedID+"/content"); body != "shared-ada" {
		t.Fatal("shared file was deleted")
	}
	if res := getStatus(t, bob, ts.URL+"/api/admin/users"); res != http.StatusForbidden {
		t.Fatalf("bob admin: %d", res)
	}

	adminFiles := listFiles(t, admin, ts.URL+"/api/admin/files")
	if _, ok := findFile(adminFiles, "a-private.txt"); !ok {
		t.Fatal("admin list missing private file")
	}
	if _, ok := findFile(adminFiles, "a-shared.txt"); !ok {
		t.Fatal("admin list missing shared file")
	}
	res = sendJSON(t, admin, http.MethodDelete, ts.URL+"/api/admin/files/"+privateID, adminCSRF, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("admin delete: %d", res.StatusCode)
	}
	res.Body.Close()
	if res := getStatus(t, ada, ts.URL+"/api/files/"+privateID+"/content"); res != http.StatusOK {
		t.Fatalf("owner preview after delete: %d", res)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "a-private.txt"); ok {
		t.Fatal("trashed file still on home")
	}
	if _, ok := findFile(listFiles(t, admin, ts.URL+"/api/admin/files"), "a-private.txt"); ok {
		t.Fatal("trashed file still in admin list")
	}

	bobUser, _, err := st.UserByUsername(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDisabled(ctx, bobUser.ID, true); err != nil {
		t.Fatal(err)
	}
	res = sendJSON(t, bob, http.MethodPost, ts.URL+"/api/login", "", map[string]string{
		"username": "bob", "password": "password1",
	})
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabled login: %d", res.StatusCode)
	}
}

func TestDuplicateBytesStoredOnce(t *testing.T) {
	ts, st := newApp(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "ada", hash, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "bob", hash, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	ada := newClient()
	bob := newClient()
	adaCSRF := login(t, ada, ts.URL, "ada", "password1")
	bobCSRF := login(t, bob, ts.URL, "bob", "password1")
	body := []byte("duplicate-body")
	upload(t, ada, ts.URL, adaCSRF, "private", "same.bin", body)
	upload(t, bob, ts.URL, bobCSRF, "private", "same.bin", body)
	usage, err := st.Usage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Files != 2 || usage.BlobBytes != int64(len(body)) {
		t.Fatalf("usage: %+v", usage)
	}

	id := fileID(t, listFiles(t, ada, ts.URL+"/api/files"), "same.bin")
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/files/"+id+"/content", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-3")
	res, err := ada.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusPartialContent || string(got) != "dupl" {
		t.Fatalf("range: %d %q", res.StatusCode, got)
	}
}

func TestShareLink(t *testing.T) {
	ts, st := newApp(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "ada", hash, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	ada := newClient()
	adaCSRF := login(t, ada, ts.URL, "ada", "password1")
	upload(t, ada, ts.URL, adaCSRF, "private", "secret.txt", []byte("via-share"))
	id := fileID(t, listFiles(t, ada, ts.URL+"/api/files"), "secret.txt")
	res := sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/"+id+"/shares", adaCSRF, map[string]string{"expires": "7d"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create share: %d", res.StatusCode)
	}
	var created struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if created.Token == "" || created.ID == 0 {
		t.Fatalf("share response: %+v", created)
	}
	anon := newClient()
	if body := get(t, anon, ts.URL+"/api/shares/"+created.Token+"/content"); body != "via-share" {
		t.Fatalf("shared body: %q", body)
	}
	if res := getStatus(t, anon, ts.URL+"/api/files"); res != http.StatusUnauthorized {
		t.Fatalf("anon library: %d", res)
	}
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+id+"/shares/"+strconv.FormatInt(created.ID, 10), adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: %d", res.StatusCode)
	}
	if res := getStatus(t, anon, ts.URL+"/api/shares/"+created.Token+"/content"); res != http.StatusNotFound {
		t.Fatalf("revoked share: %d", res)
	}
}

func newApp(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	cfg := config.Defaults()
	cfg.DataDir = t.TempDir()
	ctx := context.Background()
	st, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(slog.New(slog.DiscardHandler), st, cfg, "test", media.Binaries{})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		st.Close()
	})
	return ts, st
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func login(t *testing.T, client *http.Client, base, username, password string) string {
	t.Helper()
	res := sendJSON(t, client, http.MethodPost, base+"/api/login", "", map[string]string{
		"username": username, "password": password,
	})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, res.StatusCode, body)
	}
	var got struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(body, &got); err != nil || got.CSRF == "" {
		t.Fatalf("login csrf: %s", body)
	}
	return got.CSRF
}

func createUser(t *testing.T, client *http.Client, base, csrf, username, role string) {
	t.Helper()
	res := sendJSON(t, client, http.MethodPost, base+"/api/admin/users", csrf, map[string]string{
		"username": username, "password": "password1", "role": role,
	})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %s: %d %s", username, res.StatusCode, body)
	}
}

type listedFile struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func listFiles(t *testing.T, client *http.Client, rawURL string) []listedFile {
	t.Helper()
	var got struct {
		Files []listedFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(get(t, client, rawURL)), &got); err != nil {
		t.Fatal(err)
	}
	return got.Files
}

func findFile(files []listedFile, name string) (listedFile, bool) {
	for _, file := range files {
		if file.Name == name {
			return file, true
		}
	}
	return listedFile{}, false
}

func fileID(t *testing.T, files []listedFile, name string) string {
	t.Helper()
	file, ok := findFile(files, name)
	if !ok {
		t.Fatalf("id for %s not found", name)
	}
	return strconv.FormatInt(file.ID, 10)
}

func upload(t *testing.T, client *http.Client, base, csrf, visibility, name string, content []byte) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("visibility", visibility)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/files", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", name, res.StatusCode, body)
	}
}

func sendJSON(t *testing.T, client *http.Client, method, rawURL, csrf string, payload any) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func get(t *testing.T, client *http.Client, rawURL string) string {
	t.Helper()
	res, err := client.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", rawURL, res.StatusCode, body)
	}
	return string(body)
}

func getStatus(t *testing.T, client *http.Client, rawURL string) int {
	t.Helper()
	res, err := client.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}
