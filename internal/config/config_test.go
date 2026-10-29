package config

import "testing"

func TestLoadEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hearth.toml"
	if err := osWrite(path, "http_addr = \":1\"\ndata_dir = \"from-file\"\n"); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == "HEARTH_DATA_DIR" {
			return "from-env"
		}
		return ""
	}
	cfg, err := Load([]string{"-config", path}, getenv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":1" || cfg.DataDir != "from-env" {
		t.Fatalf("got addr %s dir %s", cfg.HTTPAddr, cfg.DataDir)
	}
}

func osWrite(path, body string) error {
	return writeFile(path, body)
}
