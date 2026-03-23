package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	messenger "github.com/slidebolt/sb-messenger-sdk"
	scriptserver "github.com/slidebolt/sb-script/server"
	storage "github.com/slidebolt/sb-storage-sdk"
	server "github.com/slidebolt/sb-storage-server"
)

type cliKey string

func (k cliKey) Key() string { return string(k) }

type cliBlob struct {
	key  string
	data json.RawMessage
}

func (b cliBlob) Key() string                  { return b.key }
func (b cliBlob) MarshalJSON() ([]byte, error) { return b.data, nil }

func TestRunStoragePullPushDeleteWithRealStore(t *testing.T) {
	msg, err := messenger.Mock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { msg.Close() })

	store, err := server.Mock(msg)
	if err != nil {
		t.Fatal(err)
	}

	key := cliKey("plugin-virtual.dev1.button")
	if err := store.WriteFile(storage.State, key, json.RawMessage(`{"type":"switch","state":{"power":false}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(storage.Profile, key, json.RawMessage(`{"labels":{"Room":["Basement"]}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(storage.Private, key, json.RawMessage(`{"secret":"v1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(storage.Internal, key, json.RawMessage(`{"cache":"v1"}`)); err != nil {
		t.Fatal(err)
	}
	scriptKey := cliKey("sb-script.scripts.party_time")
	if err := store.Save(cliBlob{key: scriptKey.Key(), data: json.RawMessage(`{"type":"script","language":"lua","name":"party_time","source":"print(\"v1\")"}`)}); err != nil {
		t.Fatal(err)
	}

	runner := NewStorageRunner(store)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()

	code := Run([]string{"storage", "pull", "--out", dir, "--pattern", ">"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("pull exit: got %d stderr=%s", code, stderr.String())
	}

	statePath := filepath.Join(dir, "plugin-virtual", "dev1", "button", "button.json")
	profilePath := filepath.Join(dir, "plugin-virtual", "dev1", "button", "button.profile.json")
	privatePath := filepath.Join(dir, "plugin-virtual", "dev1", "button", "button.private.json")
	internalPath := filepath.Join(dir, "plugin-virtual", "dev1", "button", "button.internal.json")
	luaPath := filepath.Join(dir, "sb-script", "scripts", "party_time", "party_time.lua")
	for _, path := range []string{statePath, profilePath, privatePath, internalPath, luaPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected pulled file %s: %v", path, err)
		}
	}

	stateBody, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stateBody, []byte(`"labels"`)) || bytes.Contains(stateBody, []byte(`"source"`)) {
		t.Fatalf("raw state should not be merged: %s", stateBody)
	}

	if err := os.WriteFile(statePath, []byte(`{"type":"switch","state":{"power":true}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte(`{"labels":{"Room":["Basement"],"Fixture":["Switch"]}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privatePath, []byte(`{"secret":"v2"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(internalPath, []byte(`{"cache":"v2"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(luaPath, []byte(`print("v2")`), 0644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"storage", "push", "--src", dir}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("push exit: got %d stderr=%s", code, stderr.String())
	}

	rawState, err := store.ReadFile(storage.State, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawState) != `{"type":"switch","state":{"power":true}}` {
		t.Fatalf("raw state: %s", rawState)
	}
	rawProfile, err := store.ReadFile(storage.Profile, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawProfile) != `{"labels":{"Room":["Basement"],"Fixture":["Switch"]}}` {
		t.Fatalf("raw profile: %s", rawProfile)
	}
	rawPrivate, err := store.ReadFile(storage.Private, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawPrivate) != `{"secret":"v2"}` {
		t.Fatalf("raw private: %s", rawPrivate)
	}
	rawInternal, err := store.ReadFile(storage.Internal, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawInternal) != `{"cache":"v2"}` {
		t.Fatalf("raw internal: %s", rawInternal)
	}
	rawSource, err := store.ReadFile(storage.Source, scriptKey)
	if err != nil {
		t.Fatal(err)
	}
	var source string
	if err := json.Unmarshal(rawSource, &source); err != nil {
		t.Fatal(err)
	}
	if source != `print("v2")` {
		t.Fatalf("raw source: %q", source)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"storage", "delete", "--pattern", "plugin-virtual.>"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("delete exit: got %d stderr=%s", code, stderr.String())
	}
	if _, err := store.Get(key); err == nil {
		t.Fatal("expected deleted entity to be missing")
	}
	if _, err := store.ReadFile(storage.Private, key); err == nil {
		t.Fatal("expected deleted private sidecar to be missing")
	}
}

func TestRunScriptsStartStopWithRealServices(t *testing.T) {
	msg, err := messenger.Mock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { msg.Close() })

	store, err := server.Mock(msg)
	if err != nil {
		t.Fatal(err)
	}

	scriptSvc, err := scriptserver.New(msg, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { scriptSvc.Shutdown() })

	if err := store.Save(cliBlob{key: "sb-script.scripts.party_time", data: json.RawMessage(`{"type":"script","language":"lua","name":"party_time","source":"Automation(\"party_time\", { trigger = Interval(0.05), targets = None() }, function(ctx) end)"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveQueryDefinition(store, "rgb_lights", storage.Query{
		Pattern: "test.>",
		Where:   []storage.Filter{{Field: "type", Op: storage.Eq, Value: "light"}},
	}); err != nil {
		t.Fatal(err)
	}

	runner := NewStorageRunner(store, msg)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"scripts", "start", "--name", "party_time", "--query-ref", "rgb_lights"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("script start exit: got %d stderr=%s", code, stderr.String())
	}
	var body map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	hash := body["hash"]
	if hash == "" {
		t.Fatalf("start output: %s", stdout.Bytes())
	}
	if _, err := store.Get(cliKey("sb-script.instances." + hash)); err != nil {
		t.Fatalf("expected instance: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"scripts", "stop", "--name", "party_time", "--query-ref", "rgb_lights"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("script stop exit: got %d stderr=%s", code, stderr.String())
	}
	if _, err := store.Get(cliKey("sb-script.instances." + hash)); err == nil {
		t.Fatal("expected stopped instance to be deleted")
	}
}
