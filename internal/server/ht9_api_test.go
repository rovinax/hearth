package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/store"
)

func TestHT9ThemeGroupMultiKindAPI(t *testing.T) {
	ts, st := newApp(t)
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(t.Context(), "ada", hash, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	ada := newClient()
	csrf := login(t, ada, ts.URL, "ada", "password1")

	res := sendJSON(t, ada, http.MethodPatch, ts.URL+"/api/account", csrf, map[string]string{"theme": "sakura"})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("sakura theme: %d %s", res.StatusCode, body)
	}
	var profile struct {
		User struct {
			Theme string `json:"theme"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &profile); err != nil || profile.User.Theme != "sakura" {
		t.Fatalf("profile: %s err=%v", body, err)
	}

	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/groups", csrf, map[string]string{"name": "家人"})
	group := decodeGroup(t, res)
	res = sendJSON(t, ada, http.MethodPatch, ts.URL+"/api/groups/"+itoa(group.ID), csrf, map[string]string{"name": "家族"})
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rename group: %d %s", res.StatusCode, body)
	}
	var renamed struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &renamed); err != nil || renamed.Name != "家族" {
		t.Fatalf("rename body: %s", body)
	}

	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/collections", csrf, map[string]any{
		"name": "影集", "mediaKind": "image", "mediaKinds": []string{"image", "video"},
	})
	body, _ = io.ReadAll(res.Body)
	status := res.StatusCode
	res.Body.Close()
	if status != http.StatusCreated {
		t.Fatalf("create multi: %d %s", status, body)
	}
	var col struct {
		ID         int64    `json:"id"`
		MediaKind  string   `json:"mediaKind"`
		MediaKinds []string `json:"mediaKinds"`
		Name       string   `json:"name"`
	}
	if err := json.Unmarshal(body, &col); err != nil {
		t.Fatal(err)
	}
	if col.Name != "影集" || col.MediaKind != "image" || len(col.MediaKinds) != 2 {
		t.Fatalf("col: %+v", col)
	}
	if col.MediaKinds[0] != "image" || col.MediaKinds[1] != "video" {
		t.Fatalf("kinds: %+v", col.MediaKinds)
	}

	// legacy single mediaKind still works
	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/collections", csrf, map[string]string{
		"name": "仅音频", "mediaKind": "audio",
	})
	legacy := decodeCollection(t, res)
	if legacy.ID == 0 {
		t.Fatal("legacy create failed")
	}
}
