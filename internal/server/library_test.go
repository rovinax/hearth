package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/store"
)

func TestCollectionsFavoritesAndSearch(t *testing.T) {
	ts, st := newApp(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []struct{ name, role string }{
		{"ada", store.RoleUser},
		{"bob", store.RoleUser},
		{"cara", store.RoleUser},
		{"admin", store.RoleAdmin},
	} {
		if _, err := st.CreateUser(ctx, spec.name, hash, spec.role); err != nil {
			t.Fatal(err)
		}
	}
	ada, bob, cara, admin := newClient(), newClient(), newClient(), newClient()
	adaCSRF := login(t, ada, ts.URL, "ada", "password1")
	bobCSRF := login(t, bob, ts.URL, "bob", "password1")
	caraCSRF := login(t, cara, ts.URL, "cara", "password1")
	adminCSRF := login(t, admin, ts.URL, "admin", "password1")
	_ = caraCSRF

	upload(t, ada, ts.URL, adaCSRF, "private", "small.txt", []byte("a"))
	upload(t, ada, ts.URL, adaCSRF, "private", "large.txt", bytesOf('b', 40))
	upload(t, ada, ts.URL, adaCSRF, "private", "photo.jpg", []byte("not-a-real-jpeg"))
	upload(t, ada, ts.URL, adaCSRF, "private", "海边.txt", []byte("sea"))
	upload(t, ada, ts.URL, adaCSRF, "shared", "shared-note.txt", []byte("shared-body"))

	home := listFiles(t, ada, ts.URL+"/api/files")
	if home[0].Name != "shared-note.txt" {
		t.Fatalf("default added desc: %+v", namesOf(home))
	}
	byName := listFiles(t, ada, ts.URL+"/api/files?sort=name&order=asc")
	if indexOf(byName, "large.txt") > indexOf(byName, "shared-note.txt") {
		t.Fatalf("name asc: %+v", namesOf(byName))
	}
	bySize := listFiles(t, ada, ts.URL+"/api/files?sort=size&order=asc")
	if indexOf(bySize, "small.txt") > indexOf(bySize, "large.txt") {
		t.Fatalf("size asc: %+v", namesOf(bySize))
	}
	if res := getStatus(t, ada, ts.URL+"/api/files?sort=nope"); res != http.StatusBadRequest {
		t.Fatalf("bad sort: %d", res)
	}

	smallID := fileID(t, home, "small.txt")
	largeID := fileID(t, home, "large.txt")
	photoID := fileID(t, home, "photo.jpg")
	seaID := fileID(t, home, "海边.txt")
	sharedID := fileID(t, home, "shared-note.txt")

	imageID := createCollection(t, ada, ts.URL, adaCSRF, "图片集", "image")
	if res := setCollection(t, ada, ts.URL, adaCSRF, smallID, &imageID); res != http.StatusBadRequest {
		t.Fatalf("kind mismatch: %d", res)
	}
	notesID := createCollection(t, ada, ts.URL, adaCSRF, "笔记", "other")
	if res := setCollection(t, ada, ts.URL, adaCSRF, smallID, &notesID); res != http.StatusOK {
		t.Fatalf("move in: %d", res)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "small.txt"); ok {
		t.Fatal("owner home still lists collected file")
	}
	if res := setCollection(t, ada, ts.URL, adaCSRF, sharedID, &notesID); res != http.StatusOK {
		t.Fatalf("move shared: %d", res)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "shared-note.txt"); ok {
		t.Fatal("owner home still lists collected shared file")
	}
	if _, ok := findFile(listFiles(t, bob, ts.URL+"/api/files"), "shared-note.txt"); !ok {
		t.Fatal("shared file hidden from other people")
	}
	if res := getStatus(t, bob, ts.URL+"/api/files?collection="+itoa(notesID)); res != http.StatusNotFound {
		t.Fatalf("non-member collection: %d", res)
	}
	if res := getStatus(t, bob, ts.URL+"/api/files/"+smallID+"/content"); res != http.StatusNotFound {
		t.Fatalf("private before share: %d", res)
	}

	sharedCol := decodeCollection(t, sendJSON(t, ada, http.MethodPost, ts.URL+"/api/collections/"+itoa(notesID)+"/users", adaCSRF, map[string]string{"username": "bob"}))
	if len(sharedCol.Users) != 1 || sharedCol.Users[0].Username != "bob" {
		t.Fatalf("share users: %+v", sharedCol.Users)
	}
	inCol := listFiles(t, bob, ts.URL+"/api/files?collection="+itoa(notesID))
	if _, ok := findFile(inCol, "small.txt"); !ok {
		t.Fatal("member cannot list collection")
	}
	if body := get(t, bob, ts.URL+"/api/files/"+smallID+"/content"); body != "a" {
		t.Fatalf("member download private: %q", body)
	}
	renamed := decodeCollection(t, sendJSON(t, bob, http.MethodPatch, ts.URL+"/api/collections/"+itoa(notesID), bobCSRF, map[string]string{"name": "共同笔记"}))
	if renamed.Name != "共同笔记" {
		t.Fatalf("member rename: %+v", renamed)
	}
	if res := setCollection(t, bob, ts.URL, bobCSRF, smallID, nil); res != http.StatusOK {
		t.Fatalf("member remove file: %d", res)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "small.txt"); !ok {
		t.Fatal("removed file did not return home")
	}
	res := sendJSON(t, bob, http.MethodDelete, ts.URL+"/api/collections/"+itoa(notesID), bobCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("member delete collection: %d", res.StatusCode)
	}

	res = sendJSON(t, bob, http.MethodPatch, ts.URL+"/api/files/"+sharedID, bobCSRF, map[string]string{"name": "nope.txt"})
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("member rename file: %d", res.StatusCode)
	}
	res = sendJSON(t, ada, http.MethodPatch, ts.URL+"/api/files/"+sharedID, adaCSRF, map[string]string{"name": "renamed-note.txt"})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "renamed-note.txt") {
		t.Fatalf("owner rename: %d %s", res.StatusCode, body)
	}
	res = sendJSON(t, admin, http.MethodPatch, ts.URL+"/api/files/"+largeID, adminCSRF, map[string]string{"name": "bigger.txt"})
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin rename: %d", res.StatusCode)
	}

	res = sendJSON(t, ada, http.MethodPost, ts.URL+"/api/files/"+smallID+"/favorite", adaCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("favorite: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/favorites"), "small.txt"); !ok {
		t.Fatal("favorite missing")
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "small.txt"); !ok {
		t.Fatal("favorite moved the file")
	}
	res = sendJSON(t, bob, http.MethodPost, ts.URL+"/api/files/"+smallID+"/favorite", bobCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("favorite private: %d", res.StatusCode)
	}
	if len(listFiles(t, bob, ts.URL+"/api/favorites")) != 0 {
		t.Fatal("bob sees ada favorite")
	}
	res = sendJSON(t, bob, http.MethodPost, ts.URL+"/api/files/"+sharedID+"/favorite", bobCSRF, map[string]any{})
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("favorite shared: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/favorites"), "renamed-note.txt"); ok {
		t.Fatal("ada sees bob favorite")
	}
	favOrder := listFiles(t, ada, ts.URL+"/api/favorites?sort=name&order=asc")
	if len(favOrder) != 1 || favOrder[0].Name != "small.txt" {
		t.Fatalf("favorite sort: %+v", favOrder)
	}

	group := decodeGroup(t, sendJSON(t, ada, http.MethodPost, ts.URL+"/api/groups", adaCSRF, map[string]string{"name": "家人"}))
	if len(group.Members) != 1 || group.Members[0].Username != "ada" {
		t.Fatalf("creator not member: %+v", group.Members)
	}
	group = decodeGroup(t, sendJSON(t, ada, http.MethodPost, ts.URL+"/api/groups/"+itoa(group.ID)+"/members", adaCSRF, map[string]string{"username": "cara"}))
	album := decodeCollection(t, sendJSON(t, ada, http.MethodPost, ts.URL+"/api/collections/"+itoa(imageID)+"/groups", adaCSRF, map[string]int64{"groupId": group.ID}))
	if len(album.Groups) != 1 {
		t.Fatalf("share group: %+v", album.Groups)
	}
	if res := setCollection(t, ada, ts.URL, adaCSRF, photoID, &imageID); res != http.StatusOK {
		t.Fatalf("photo into album: %d", res)
	}
	if res := getStatus(t, cara, ts.URL+"/api/files/"+photoID+"/content"); res != http.StatusOK {
		t.Fatalf("group member open private: %d", res)
	}
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/groups/"+itoa(group.ID), adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete group: %d", res.StatusCode)
	}
	if res := getStatus(t, ada, ts.URL+"/api/collections/"+itoa(imageID)); res != http.StatusOK {
		t.Fatalf("collection deleted with group: %d", res)
	}
	if res := getStatus(t, cara, ts.URL+"/api/files/"+photoID+"/content"); res != http.StatusNotFound {
		t.Fatalf("left member still opens file: %d", res)
	}

	adminBody := get(t, admin, ts.URL+"/api/admin/files")
	if !strings.Contains(adminBody, "共同笔记") || !strings.Contains(adminBody, "renamed-note.txt") {
		t.Fatalf("admin list missing collection: %s", adminBody)
	}

	found := decodeSearch(t, ada, ts.URL+"/api/search?q="+urlQuery("海边"))
	if _, ok := findFile(found.Files, "海边.txt"); !ok {
		t.Fatalf("search files: %+v", namesOf(found.Files))
	}
	seaCol := createCollection(t, ada, ts.URL, adaCSRF, "海边相册", "other")
	found = decodeSearch(t, ada, ts.URL+"/api/search?q="+urlQuery("海边"))
	if !hasCollection(found.Collections, seaCol) {
		t.Fatalf("search collections: %+v", found.Collections)
	}
	if _, ok := findFile(found.Files, "海边.txt"); !ok {
		t.Fatal("search missed file after second query")
	}
	history := listHistory(t, ada, ts.URL)
	if len(history) != 1 || history[0].Query != "海边" {
		t.Fatalf("deduped history: %+v", history)
	}
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/search/history/"+itoa(history[0].ID), adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete history: %d", res.StatusCode)
	}
	if len(listHistory(t, ada, ts.URL)) != 0 {
		t.Fatal("history remains")
	}
	for i := 0; i < 21; i++ {
		decodeSearch(t, ada, ts.URL+"/api/search?q=q"+strconv.Itoa(i))
	}
	history = listHistory(t, ada, ts.URL)
	if len(history) != 20 {
		t.Fatalf("history cap: %d", len(history))
	}
	for _, item := range history {
		if item.Query == "q0" {
			t.Fatal("oldest query kept")
		}
	}
	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/search/history", adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent || len(listHistory(t, ada, ts.URL)) != 0 {
		t.Fatal("clear history")
	}
	if res := getStatus(t, bob, ts.URL+"/api/search?q="+urlQuery("海边")); res != http.StatusOK {
		t.Fatalf("bob search: %d", res)
	}
	bobFound := decodeSearch(t, bob, ts.URL+"/api/search?q="+urlQuery("海边"))
	if _, ok := findFile(bobFound.Files, "海边.txt"); ok || hasCollection(bobFound.Collections, seaCol) {
		t.Fatal("bob sees private search hits")
	}

	res = sendJSON(t, ada, http.MethodDelete, ts.URL+"/api/collections/"+itoa(notesID), adaCSRF, nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("owner delete collection: %d", res.StatusCode)
	}
	if _, ok := findFile(listFiles(t, ada, ts.URL+"/api/files"), "renamed-note.txt"); !ok {
		t.Fatal("delete collection did not restore file")
	}
	_ = seaID
}

func bytesOf(b byte, n int) []byte {
	return []byte(strings.Repeat(string(b), n))
}

func namesOf(files []listedFile) []string {
	out := make([]string, len(files))
	for i, file := range files {
		out[i] = file.Name
	}
	return out
}

func indexOf(files []listedFile, name string) int {
	for i, file := range files {
		if file.Name == name {
			return i
		}
	}
	return -1
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

func urlQuery(q string) string { return url.QueryEscape(q) }

func createCollection(t *testing.T, client *http.Client, base, csrf, name, kind string) int64 {
	t.Helper()
	item := decodeCollection(t, sendJSON(t, client, http.MethodPost, base+"/api/collections", csrf, map[string]string{
		"name": name, "mediaKind": kind,
	}))
	if item.ID == 0 || item.Name != name {
		t.Fatalf("create collection: %+v", item)
	}
	return item.ID
}

func setCollection(t *testing.T, client *http.Client, base, csrf, fileID string, collectionID *int64) int {
	t.Helper()
	res := sendJSON(t, client, http.MethodPost, base+"/api/files/"+fileID+"/collection", csrf, map[string]any{
		"collectionId": collectionID,
	})
	res.Body.Close()
	return res.StatusCode
}

type listedCollection struct {
	ID     int64       `json:"id"`
	Name   string      `json:"name"`
	Users  []userBrief `json:"users"`
	Groups []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"groups"`
}

type userBrief struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type listedGroup struct {
	ID      int64       `json:"id"`
	Name    string      `json:"name"`
	Members []userBrief `json:"members"`
}

func decodeCollection(t *testing.T, res *http.Response) listedCollection {
	t.Helper()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		t.Fatalf("collection %d %s", res.StatusCode, body)
	}
	var item listedCollection
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func decodeGroup(t *testing.T, res *http.Response) listedGroup {
	t.Helper()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		t.Fatalf("group %d %s", res.StatusCode, body)
	}
	var item listedGroup
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

type searchBody struct {
	Files       []listedFile       `json:"files"`
	Collections []listedCollection `json:"collections"`
}

func decodeSearch(t *testing.T, client *http.Client, rawURL string) searchBody {
	t.Helper()
	var got searchBody
	if err := json.Unmarshal([]byte(get(t, client, rawURL)), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func hasCollection(items []listedCollection, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

type historyItem struct {
	ID    int64  `json:"id"`
	Query string `json:"query"`
}

func listHistory(t *testing.T, client *http.Client, base string) []historyItem {
	t.Helper()
	var got struct {
		History []historyItem `json:"history"`
	}
	if err := json.Unmarshal([]byte(get(t, client, base+"/api/search/history")), &got); err != nil {
		t.Fatal(err)
	}
	return got.History
}
