package app

import (
	"bytes"
	"encoding/json"
	"testing"

	storage "github.com/slidebolt/sb-storage-sdk"
)

type fakeRunner struct {
	pullOpts      []storage.PullProfilesOptions
	planOpts      []storage.ProfileRestoreOptions
	applyOpts     []storage.ProfileRestoreOptions
	storagePull   []struct{ outDir, pattern string }
	storagePush   []string
	storageDelete []string
	startScript   []struct{ name, queryRef string }
	stopScript    []struct{ name, queryRef string }
	startHash     string
	plan          storage.RestorePlan
	err           error
}

func (f *fakeRunner) PullProfiles(opts storage.PullProfilesOptions) error {
	f.pullOpts = append(f.pullOpts, opts)
	return f.err
}

func (f *fakeRunner) PlanProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error) {
	f.planOpts = append(f.planOpts, opts)
	return f.plan, f.err
}

func (f *fakeRunner) ApplyProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error) {
	f.applyOpts = append(f.applyOpts, opts)
	return f.plan, f.err
}

func (f *fakeRunner) PullStorage(outDir, pattern string) error {
	f.storagePull = append(f.storagePull, struct{ outDir, pattern string }{outDir: outDir, pattern: pattern})
	return f.err
}

func (f *fakeRunner) PushStorage(srcDir string) error {
	f.storagePush = append(f.storagePush, srcDir)
	return f.err
}

func (f *fakeRunner) DeleteStorage(pattern string) error {
	f.storageDelete = append(f.storageDelete, pattern)
	return f.err
}

func (f *fakeRunner) StartScript(name, queryRef string) (string, error) {
	f.startScript = append(f.startScript, struct{ name, queryRef string }{name: name, queryRef: queryRef})
	return f.startHash, f.err
}

func (f *fakeRunner) StopScript(name, queryRef string) error {
	f.stopScript = append(f.stopScript, struct{ name, queryRef string }{name: name, queryRef: queryRef})
	return f.err
}

func TestRunBackupPullPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"backup", "pull", "--out", "/tmp/out", "--pattern", "plugin.>"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.pullOpts) != 1 {
		t.Fatalf("pull calls: got %d want 1", len(runner.pullOpts))
	}
	if runner.pullOpts[0].OutDir != "/tmp/out" || runner.pullOpts[0].Pattern != "plugin.>" {
		t.Fatalf("pull opts: %+v", runner.pullOpts[0])
	}
}

func TestRunRestorePlanPrintsPlanJSON(t *testing.T) {
	runner := &fakeRunner{
		plan: storage.RestorePlan{
			Operations: []storage.RestoreOperation{
				{Key: "plugin.device.entity", Action: storage.RestoreActionCreate},
			},
		},
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"restore", "plan", "--src", "/tmp/src"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.planOpts) != 1 || runner.planOpts[0].SourceDir != "/tmp/src" {
		t.Fatalf("plan opts: %+v", runner.planOpts)
	}
	var plan storage.RestorePlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 1 || plan.Operations[0].Key != "plugin.device.entity" {
		t.Fatalf("plan output: %+v", plan)
	}
}

func TestRunRestoreApplyPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{plan: storage.RestorePlan{}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"restore", "apply", "--src", "/tmp/src"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.applyOpts) != 1 || runner.applyOpts[0].SourceDir != "/tmp/src" {
		t.Fatalf("apply opts: %+v", runner.applyOpts)
	}
}

func TestRunStoragePullPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"storage", "pull", "--out", "/tmp/storage", "--pattern", "plugin.>"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.storagePull) != 1 || runner.storagePull[0].outDir != "/tmp/storage" || runner.storagePull[0].pattern != "plugin.>" {
		t.Fatalf("storage pull opts: %+v", runner.storagePull)
	}
}

func TestRunStoragePushPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"storage", "push", "--src", "/tmp/storage"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.storagePush) != 1 || runner.storagePush[0] != "/tmp/storage" {
		t.Fatalf("storage push opts: %+v", runner.storagePush)
	}
}

func TestRunStorageDeletePassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"storage", "delete", "--pattern", "plugin.>"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.storageDelete) != 1 || runner.storageDelete[0] != "plugin.>" {
		t.Fatalf("storage delete opts: %+v", runner.storageDelete)
	}
}

func TestRunScriptsStartPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{startHash: "abc123"}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"scripts", "start", "--name", "PartyTime", "--query-ref", "rgb_lights"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.startScript) != 1 || runner.startScript[0].name != "PartyTime" || runner.startScript[0].queryRef != "rgb_lights" {
		t.Fatalf("start script opts: %+v", runner.startScript)
	}
	var body map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["hash"] != "abc123" {
		t.Fatalf("start output: %+v", body)
	}
}

func TestRunScriptsStopPassesFlagsThrough(t *testing.T) {
	runner := &fakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"scripts", "stop", "--name", "PartyTime", "--query-ref", "rgb_lights"}, stdout, stderr, runner)
	if code != 0 {
		t.Fatalf("exit: got %d stderr=%s", code, stderr.String())
	}
	if len(runner.stopScript) != 1 || runner.stopScript[0].name != "PartyTime" || runner.stopScript[0].queryRef != "rgb_lights" {
		t.Fatalf("stop script opts: %+v", runner.stopScript)
	}
}
