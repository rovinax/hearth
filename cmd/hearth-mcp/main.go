package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"hearth/internal/hearthapi"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "hearth-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	base := envOr("HEARTH_BASE_URL", "http://127.0.0.1:8080")
	user := strings.TrimSpace(os.Getenv("HEARTH_USERNAME"))
	pass := os.Getenv("HEARTH_PASSWORD")
	if user == "" || pass == "" {
		fmt.Fprintf(os.Stderr, "hearth-mcp: warning: HEARTH_USERNAME/HEARTH_PASSWORD unset; tool calls will fail until set\n")
	}
	client, err := hearthapi.New(base, user, pass)
	if err != nil {
		return err
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "hearth", Version: "0.1.0"}, nil)
	registerTools(srv, client)
	return srv.Run(context.Background(), &mcp.StdioTransport{})
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolErr(err), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func rawResult(raw json.RawMessage) (*mcp.CallToolResult, any, error) {
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		}, nil, nil
	}
	return jsonResult(pretty)
}

func toolErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func registerTools(srv *mcp.Server, client *hearthapi.Client) {
	type emptyArgs struct{}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_whoami",
		Description: "Return the logged-in Hearth account (GET /api/me). Use to confirm auth and read profile fields.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		raw, err := client.Get("/api/me", nil)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type listFilesArgs struct {
		Collection int64  `json:"collection,omitempty" jsonschema:"optional collection id to list files inside"`
		Kind       string `json:"kind,omitempty" jsonschema:"optional media kind filter: photo, video, or other"`
		Trash      bool   `json:"trash,omitempty" jsonschema:"if true, list trash instead of library"`
		Sort       string `json:"sort,omitempty" jsonschema:"optional sort key as accepted by Hearth API"`
		Order      string `json:"order,omitempty" jsonschema:"optional asc or desc"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_list_files",
		Description: "List library files (GET /api/files). Optional filters: collection id, kind (photo|video|other), trash, sort, order.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listFilesArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		if args.Collection > 0 {
			q.Set("collection", strconv.FormatInt(args.Collection, 10))
		}
		if args.Kind != "" {
			q.Set("kind", args.Kind)
		}
		if args.Trash {
			q.Set("trash", "1")
		}
		if args.Sort != "" {
			q.Set("sort", args.Sort)
		}
		if args.Order != "" {
			q.Set("order", args.Order)
		}
		raw, err := client.Get("/api/files", q)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type searchArgs struct {
		Q     string `json:"q" jsonschema:"search query text"`
		Sort  string `json:"sort,omitempty" jsonschema:"optional sort key"`
		Order string `json:"order,omitempty" jsonschema:"optional asc or desc"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_search",
		Description: "Search files and collections by text (GET /api/search?q=...). Returns matching files and collections.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Q) == "" {
			return toolErr(fmt.Errorf("q is required")), nil, nil
		}
		q := url.Values{"q": {args.Q}}
		if args.Sort != "" {
			q.Set("sort", args.Sort)
		}
		if args.Order != "" {
			q.Set("order", args.Order)
		}
		raw, err := client.Get("/api/search", q)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type listCollectionsArgs struct {
		Kind string `json:"kind,omitempty" jsonschema:"optional media kind filter: photo, video, or other"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_list_collections",
		Description: "List collections/sets the user can see (GET /api/collections).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listCollectionsArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		if args.Kind != "" {
			q.Set("kind", args.Kind)
		}
		raw, err := client.Get("/api/collections", q)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type createCollectionArgs struct {
		Name       string   `json:"name" jsonschema:"collection display name"`
		MediaKind  string   `json:"mediaKind,omitempty" jsonschema:"single media kind: photo, video, or other (legacy)"`
		MediaKinds []string `json:"mediaKinds,omitempty" jsonschema:"one or more media kinds, e.g. [\"photo\",\"video\"]"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_create_collection",
		Description: "Create a collection/set (POST /api/collections). Prefer mediaKinds for multi-kind sets (photo+video).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args createCollectionArgs) (*mcp.CallToolResult, any, error) {
		body := map[string]any{"name": args.Name}
		if args.MediaKind != "" {
			body["mediaKind"] = args.MediaKind
		}
		if len(args.MediaKinds) > 0 {
			body["mediaKinds"] = args.MediaKinds
		}
		raw, err := client.Mutate(httpMethodPost, "/api/collections", body)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_list_groups",
		Description: "List groups the user belongs to or owns (GET /api/groups).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		raw, err := client.Get("/api/groups", nil)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type renameGroupArgs struct {
		ID   int64  `json:"id" jsonschema:"group id"`
		Name string `json:"name" jsonschema:"new non-empty group name"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_rename_group",
		Description: "Rename a group you own (PATCH /api/groups/{id}). Requires owner permission.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args renameGroupArgs) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 || strings.TrimSpace(args.Name) == "" {
			return toolErr(fmt.Errorf("id and name are required")), nil, nil
		}
		raw, err := client.Mutate(httpMethodPatch, "/api/groups/"+strconv.FormatInt(args.ID, 10), map[string]string{"name": args.Name})
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type fileIDArgs struct {
		ID int64 `json:"id" jsonschema:"file id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_file_metadata",
		Description: "Get one file's metadata including shares if editable (GET /api/files/{id}).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args fileIDArgs) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolErr(fmt.Errorf("id is required")), nil, nil
		}
		raw, err := client.Get("/api/files/"+strconv.FormatInt(args.ID, 10), nil)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type favoriteArgs struct {
		ID       int64 `json:"id" jsonschema:"file id"`
		Favorite bool  `json:"favorite" jsonschema:"true to favorite, false to unfavorite"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_set_favorite",
		Description: "Favorite or unfavorite a file (POST or DELETE /api/files/{id}/favorite).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args favoriteArgs) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolErr(fmt.Errorf("id is required")), nil, nil
		}
		path := "/api/files/" + strconv.FormatInt(args.ID, 10) + "/favorite"
		method := httpMethodPost
		if !args.Favorite {
			method = httpMethodDelete
		}
		raw, err := client.Mutate(method, path, nil)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

	type shareArgs struct {
		ID      int64  `json:"id" jsonschema:"file id"`
		Expires string `json:"expires,omitempty" jsonschema:"share expiry: never (default), 7d, or 30d"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hearth_create_share",
		Description: "Create a public share link for a file you can edit (POST /api/files/{id}/shares). expires: never|7d|30d.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args shareArgs) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolErr(fmt.Errorf("id is required")), nil, nil
		}
		expires := args.Expires
		if expires == "" {
			expires = "never"
		}
		raw, err := client.Mutate(httpMethodPost, "/api/files/"+strconv.FormatInt(args.ID, 10)+"/shares", map[string]string{"expires": expires})
		if err != nil {
			return toolErr(err), nil, nil
		}
		return rawResult(raw)
	})

}

const (
	httpMethodPost   = "POST"
	httpMethodPatch  = "PATCH"
	httpMethodDelete = "DELETE"
)

