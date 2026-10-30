package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/store"
)

func TestBatchAndProfile(t *testing.T) {
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
	csrf := login(t, ada, ts.URL, "ada", "password1")
	upload(t, ada, ts.URL, csrf, "private", "one.txt", []byte("one"))
	upload(t, ada, ts.URL, csrf, "private", "two.txt", []byte("two"))
	home := listFiles(t, ada, ts.URL+"/api/files")
	one := mustFile(t, home, "one.txt").ID
	two := mustFile(t, home, "two.txt").ID
	notes := createCollection(t, ada, ts.URL, csrf, "笔记", "other")

	res := sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/batch", csrf, map[string]any{
		"ids": []int64{one, two}, "action": "move", "collectionId": notes,
	})
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("batch move: %d %s", res.StatusCode, body)
	}
	res.Body.Close()
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "one.txt"); ok {
		t.Fatal("moved file still on home")
	}
	inside := listFiles(t, ada, ts.URL+"/api/files?collection="+itoa(notes))
	if len(inside) != 2 {
		t.Fatalf("collection files: %d", len(inside))
	}

	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/batch", csrf, map[string]any{
		"ids": []int64{one, two}, "action": "copy", "collectionId": nil,
	})
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("batch copy: %d %s", res.StatusCode, body)
	}
	res.Body.Close()
	copied := listFiles(t, ada, ts.URL+"/api/files")
	if len(copied) != 2 {
		t.Fatalf("home copies: %+v", namesOf(copied))
	}
	if len(listFiles(t, ada, ts.URL+"/api/files?collection="+itoa(notes))) != 2 {
		t.Fatal("copy removed originals from collection")
	}

	ids := []int64{copied[0].ID, copied[1].ID}
	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/batch", csrf, map[string]any{
		"ids": ids, "action": "delete",
	})
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("batch delete: %d %s", res.StatusCode, body)
	}
	res.Body.Close()
	if len(listFiles(t, ada, ts.URL+"/api/files")) != 0 {
		t.Fatal("deleted copies still on home")
	}
	if len(listFiles(t, ada, ts.URL+"/api/files?trash=1")) != 2 {
		t.Fatal("trash missing copies")
	}

	res = sendJSON(t, ada, http.MethodPatch, ts.URL+"/api/account", csrf, map[string]string{
		"displayName": "阿达", "bio": "在家", "theme": "pine",
	})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("profile: %d %s", res.StatusCode, body)
	}
	var profile struct {
		User struct {
			DisplayName string `json:"displayName"`
			Theme       string `json:"theme"`
			Bio         string `json:"bio"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.User.DisplayName != "阿达" || profile.User.Theme != "pine" || profile.User.Bio != "在家" {
		t.Fatalf("profile body: %+v", profile.User)
	}
	res = sendJSON(t, ada, http.MethodPatch, ts.URL+"/api/account", csrf, map[string]string{"theme": "nope"})
	if res.StatusCode != http.StatusBadRequest {
		res.Body.Close()
		t.Fatalf("bad theme: %d", res.StatusCode)
	}
	res.Body.Close()

	pngBody := pngBytes(t)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(pngBody); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/account/avatar", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	res, err = ada.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("avatar: %d %s", res.StatusCode, body)
	}
	var me struct {
		User struct {
			ID        int64  `json:"id"`
			AvatarURL string `json:"avatarUrl"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &me); err != nil {
		t.Fatal(err)
	}
	if me.User.AvatarURL == "" {
		t.Fatal("missing avatar url")
	}
	got := getStatus(t, ada, ts.URL+me.User.AvatarURL)
	if got != http.StatusOK {
		t.Fatalf("avatar get: %d", got)
	}
}

func mustFile(t *testing.T, files []listedFile, name string) listedFile {
	t.Helper()
	file, ok := findFile(files, name)
	if !ok {
		t.Fatalf("missing %s", name)
	}
	return file
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
