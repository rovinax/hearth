package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"hearth/internal/auth"
	"hearth/internal/store"
)

func TestTrash(t *testing.T) {
	ts, st := newApp(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []struct{ name, role string }{
		{"ada", store.RoleUser},
		{"bob", store.RoleUser},
		{"admin", store.RoleAdmin},
	} {
		if _, err := st.CreateUser(ctx, spec.name, hash, spec.role); err != nil {
			t.Fatal(err)
		}
	}
	ada, bob, admin := newClient(), newClient(), newClient()
	adaCSRF := login(t, ada, ts.URL, "ada", "password1")
	bobCSRF := login(t, bob, ts.URL, "bob", "password1")
	adminCSRF := login(t, admin, ts.URL, "admin", "password1")

	body := []byte("hello-trash")
	upload(t, ada, ts.URL, adaCSRF, "private", "note.txt", body)
	upload(t, ada, ts.URL, adaCSRF, "private", "copy.txt", body)
	home := listFiles(t, ada, ts.URL+"/api/files")
	noteID := fileID(t, home, "note.txt")
	copyID := fileID(t, home, "copy.txt")
	notes := createCollection(t, ada, ts.URL, adaCSRF, "笔记", "other")
	if res := setCollection(t, ada, ts.URL, adaCSRF, noteID, &notes); res != http.StatusOK {
		t.Fatalf("move in: %d", res)
	}
	res := sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/"+noteID+"/favorite", adaCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		t.Fatalf("favorite: %d", res.StatusCode)
	}
	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/"+noteID+"/shares", adaCSRF, map[string]string{"expires": "7d"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("share: %d", res.StatusCode)
	}
	var share struct {
		Token string `json:"token"`
	}
	if err := jsonNew(res, &share); err != nil || share.Token == "" {
		t.Fatalf("share token: %v", err)
	}
	anon := newClient()

	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+noteID, adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "note.txt"); ok {
		t.Fatal("home still lists deleted file")
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files?collection="+itoa(notes)), "note.txt"); ok {
		t.Fatal("collection still lists deleted file")
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/favorites"), "note.txt"); ok {
		t.Fatal("favorites still lists deleted file")
	}
	if _, ok := findFile(decodeSearch(t, ada, ts.URL+"/api/search?q=note").Files, "note.txt"); ok {
		t.Fatal("search still lists deleted file")
	}
	if res := getStatus(t, anon, ts.URL+"/api/shares/"+share.Token+"/content"); res != http.StatusNotFound {
		t.Fatalf("share while trashed: %d", res)
	}
	if _, ok := findFile(listFiles(t, bob, ts.URL+"/api/files?trash=1"), "note.txt"); ok {
		t.Fatal("bob sees ada trash")
	}
	res = sendJSON(t, bob, http.MethodPost, ts.URL+"/api/files/"+noteID+"/restore", bobCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("bob restore: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files?trash=1"), "note.txt"); !ok {
		t.Fatal("owner trash missing file")
	}
	if _, ok := findFile(listFiles(t, admin, ts.URL+"/api/files?trash=1"), "note.txt"); !ok {
		t.Fatal("admin trash missing file")
	}
	if res := getStatus(t, ada, ts.URL+"/api/files/"+noteID+"/content"); res != http.StatusOK {
		t.Fatalf("owner preview: %d", res)
	}
	if res := getStatus(t, bob, ts.URL+"/api/files/"+noteID+"/content"); res != http.StatusNotFound {
		t.Fatalf("bob preview: %d", res)
	}

	res = sendJSON(t, admin, http.MethodPost, ts.URL+"/api/files/"+noteID+"/restore", adminCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin restore: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "note.txt"); ok {
		t.Fatal("restored file returned to home instead of the collection")
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files?collection="+itoa(notes)), "note.txt"); !ok {
		t.Fatal("restore did not return the file to its collection")
	}
	if body := get(t, anon, ts.URL+"/api/shares/"+share.Token+"/content"); body != "hello-trash" {
		t.Fatalf("share after restore: %q", body)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/favorites"), "note.txt"); !ok {
		t.Fatal("favorite was dropped")
	}

	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+noteID, adaCSRF, nil)
	res.Body.Close()
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/collections/"+itoa(notes), adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete collection: %d", res.StatusCode)
	}
	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/"+noteID+"/restore", adaCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("restore after collection gone: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "note.txt"); !ok {
		t.Fatal("restore did not return the file home")
	}

	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+noteID, adaCSRF, nil)
	res.Body.Close()
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+noteID+"/purge", adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("purge: %d", res.StatusCode)
	}
	if res := getStatus(t, ada, ts.URL+"/api/files/"+noteID+"/content"); res != http.StatusNotFound {
		t.Fatalf("purged content: %d", res)
	}
	if _, err := st.GetFile(ctx, mustID(t, noteID)); err == nil || err != store.ErrNotFound {
		t.Fatalf("purged row: %v", err)
	}
	if body := get(t, ada, ts.URL+"/api/files/"+copyID+"/content"); body != "hello-trash" {
		t.Fatalf("shared blob removed: %q", body)
	}

	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/files/"+copyID, adaCSRF, nil)
	res.Body.Close()
	removed, err := st.PurgeExpired(ctx, time.Now().Add(31*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0].Rel == "" {
		t.Fatalf("expired blob: %+v", removed)
	}
	if _, err := st.GetFile(ctx, mustID(t, copyID)); err != store.ErrNotFound {
		t.Fatalf("expired row: %v", err)
	}
}

func jsonNew(res *http.Response, dst any) error {
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(dst)
}

func mustID(t *testing.T, raw string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
