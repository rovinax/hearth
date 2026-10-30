package server

import (
	"net/http"
	"strconv"

	"hearth/internal/store"
)

type collectionJSON struct {
	ID         int64        `json:"id"`
	Name       string       `json:"name"`
	MediaKind  string       `json:"mediaKind"`
	MediaKinds []string     `json:"mediaKinds"`
	OwnerID    int64        `json:"ownerId"`
	OwnerName  string       `json:"ownerName"`
	CanDelete  bool         `json:"canDelete"`
	CreatedAt  string       `json:"createdAt"`
	Users      []userBrief  `json:"users,omitempty"`
	Groups     []groupBrief `json:"groups,omitempty"`
}

type userBrief struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type groupBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type groupJSON struct {
	ID        int64       `json:"id"`
	Name      string      `json:"name"`
	OwnerID   int64       `json:"ownerId"`
	OwnerName string      `json:"ownerName"`
	CanManage bool        `json:"canManage"`
	CreatedAt string      `json:"createdAt"`
	Members   []userBrief `json:"members"`
}

type historyJSON struct {
	ID        int64  `json:"id"`
	Query     string `json:"query"`
	CreatedAt string `json:"createdAt"`
}

func collectionOut(item store.Collection, actor store.User) collectionJSON {
	kinds := append([]string{}, item.MediaKinds...)
	if len(kinds) == 0 && item.MediaKind != "" {
		kinds = []string{item.MediaKind}
	}
	out := collectionJSON{
		ID: item.ID, Name: item.Name, MediaKind: item.MediaKind, MediaKinds: kinds,
		OwnerID: item.OwnerID, OwnerName: item.OwnerName,
		CanDelete: actor.ID == item.OwnerID, CreatedAt: item.CreatedAt,
	}
	if len(item.Users) > 0 {
		out.Users = make([]userBrief, 0, len(item.Users))
		for _, user := range item.Users {
			out.Users = append(out.Users, userBrief{ID: user.ID, Username: user.Username})
		}
	}
	if len(item.Groups) > 0 {
		out.Groups = make([]groupBrief, 0, len(item.Groups))
		for _, group := range item.Groups {
			out.Groups = append(out.Groups, groupBrief{ID: group.ID, Name: group.Name})
		}
	}
	return out
}

func collectionsOut(items []store.Collection, actor store.User) []collectionJSON {
	out := make([]collectionJSON, 0, len(items))
	for _, item := range items {
		out = append(out, collectionOut(item, actor))
	}
	return out
}

func groupOut(item store.Group, actor store.User) groupJSON {
	out := groupJSON{
		ID: item.ID, Name: item.Name, OwnerID: item.OwnerID, OwnerName: item.OwnerName,
		CanManage: actor.ID == item.OwnerID, CreatedAt: item.CreatedAt,
		Members: make([]userBrief, 0, len(item.Members)),
	}
	for _, user := range item.Members {
		out.Members = append(out.Members, userBrief{ID: user.ID, Username: user.Username})
	}
	return out
}

func (s *Server) apiCollections(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	items, err := s.store.ListCollections(r.Context(), user, cleanKind(r.URL.Query().Get("kind")))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": collectionsOut(items, user)})
}

func (s *Server) apiCreateCollection(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		Name       string   `json:"name"`
		MediaKind  string   `json:"mediaKind"`
		MediaKinds []string `json:"mediaKinds"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	kinds := append([]string{}, body.MediaKinds...)
	if body.MediaKind != "" {
		found := false
		for _, kind := range kinds {
			if kind == body.MediaKind {
				found = true
				break
			}
		}
		if !found {
			kinds = append([]string{body.MediaKind}, kinds...)
		}
	}
	item, err := s.store.CreateCollection(r.Context(), user, body.Name, kinds)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, collectionOut(item, user))
}

func (s *Server) apiCollection(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	item, err := s.store.GetCollection(r.Context(), id, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiRenameCollection(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.RenameCollection(r.Context(), id, user, body.Name)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiDeleteCollection(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteCollection(r.Context(), id, user); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiShareCollectionUser(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.ShareCollectionUser(r.Context(), id, user, body.Username)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiUnshareCollectionUser(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	item, err := s.store.UnshareCollectionUser(r.Context(), id, userID, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiShareCollectionGroup(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		GroupID int64 `json:"groupId"`
	}
	if err := readJSON(r, &body); err != nil || body.GroupID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.ShareCollectionGroup(r.Context(), id, body.GroupID, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiUnshareCollectionGroup(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	groupID, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	item, err := s.store.UnshareCollectionGroup(r.Context(), id, groupID, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collectionOut(item, user))
}

func (s *Server) apiBatchFiles(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		IDs          []int64 `json:"ids"`
		Action       string  `json:"action"`
		CollectionID *int64  `json:"collectionId"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(body.IDs))
	for _, id := range body.IDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	var collectionID int64
	if body.CollectionID != nil {
		collectionID = *body.CollectionID
	}
	var removed []store.RemovedFile
	for _, id := range ids {
		switch body.Action {
		case "move":
			if _, err := s.store.SetFileCollection(r.Context(), user, id, collectionID); err != nil {
				writeStoreErr(w, err)
				return
			}
		case "copy":
			if _, err := s.store.CopyFile(r.Context(), user, id, collectionID); err != nil {
				writeStoreErr(w, err)
				return
			}
		case "delete":
			rel, thumb, err := s.store.DeleteFile(r.Context(), id, user)
			if err != nil {
				writeStoreErr(w, err)
				return
			}
			if rel != "" || thumb != "" {
				removed = append(removed, store.RemovedFile{Rel: rel, Thumb: thumb})
			}
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
			return
		}
	}
	s.dropRemoved(removed)
	writeJSON(w, http.StatusOK, map[string]int{"count": len(ids)})
}

func (s *Server) apiSetCollection(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		CollectionID *int64 `json:"collectionId"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	var collectionID int64
	if body.CollectionID != nil {
		collectionID = *body.CollectionID
	}
	file, err := s.store.SetFileCollection(r.Context(), user, id, collectionID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fileOut(file, user))
}

func (s *Server) apiRenameFile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	file, err := s.store.RenameFile(r.Context(), user, id, body.Name)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fileOut(file, user))
}

func (s *Server) apiFavorite(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.AddFavorite(r.Context(), user, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiUnfavorite(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.RemoveFavorite(r.Context(), user, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiFavorites(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	files, err := s.store.ListFavorites(r.Context(), user, cleanKind(r.URL.Query().Get("kind")), r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": filesOut(files, user)})
}

func (s *Server) apiGroups(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	items, err := s.store.ListGroups(r.Context(), user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]groupJSON, 0, len(items))
	for _, item := range items {
		out = append(out, groupOut(item, user))
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

func (s *Server) apiCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.CreateGroup(r.Context(), user, body.Name)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, groupOut(item, user))
}

func (s *Server) apiRenameGroup(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.RenameGroup(r.Context(), id, user, body.Name)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groupOut(item, user))
}

func (s *Server) apiDeleteGroup(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteGroup(r.Context(), id, user); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiAddGroupMember(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	item, err := s.store.AddGroupMember(r.Context(), id, user, body.Username)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groupOut(item, user))
}

func (s *Server) apiRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	item, err := s.store.RemoveGroupMember(r.Context(), id, userID, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if user.ID == userID {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, groupOut(item, user))
}

func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	files, collections, err := s.store.Search(r.Context(), user, r.URL.Query().Get("q"), r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":       r.URL.Query().Get("q"),
		"files":       filesOut(files, user),
		"collections": collectionsOut(collections, user),
	})
}

func (s *Server) apiSearchHistory(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	items, err := s.store.ListSearchHistory(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]historyJSON, 0, len(items))
	for _, item := range items {
		out = append(out, historyJSON{ID: item.ID, Query: item.Query, CreatedAt: item.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": out})
}

func (s *Server) apiDeleteSearchHistory(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteSearchHistory(r.Context(), user.ID, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiClearSearchHistory(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	if err := s.store.ClearSearchHistory(r.Context(), user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return 0, false
	}
	return id, true
}
