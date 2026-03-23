package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"
)

type Runner interface {
	PullProfiles(opts storage.PullProfilesOptions) error
	PlanProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error)
	ApplyProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error)
	PullStorage(outDir, pattern string) error
	PushStorage(srcDir string) error
	DeleteStorage(pattern string) error
	StartScript(name, queryRef string) (string, error)
	StopScript(name, queryRef string) error
}

func Run(args []string, stdout, stderr io.Writer, runner Runner) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "backup":
		return runBackup(args[1:], stderr, runner)
	case "restore":
		return runRestore(args[1:], stdout, stderr, runner)
	case "storage":
		return runStorage(args[1:], stderr, runner)
	case "scripts":
		return runScripts(args[1:], stdout, stderr, runner)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: sb-cli backup pull --out DIR [--pattern PATTERN]")
	fmt.Fprintln(w, "       sb-cli restore plan --src DIR")
	fmt.Fprintln(w, "       sb-cli restore apply --src DIR")
	fmt.Fprintln(w, "       sb-cli storage pull --out DIR [--pattern PATTERN]")
	fmt.Fprintln(w, "       sb-cli storage push --src DIR")
	fmt.Fprintln(w, "       sb-cli storage delete --pattern PATTERN")
	fmt.Fprintln(w, "       sb-cli scripts start --name NAME --query-ref NAME")
	fmt.Fprintln(w, "       sb-cli scripts stop --name NAME --query-ref NAME")
}

func runBackup(args []string, stderr io.Writer, runner Runner) int {
	if len(args) == 0 || args[0] != "pull" {
		usage(stderr)
		return 2
	}
	fs := flag.NewFlagSet("backup pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outDir := fs.String("out", "", "output directory")
	pattern := fs.String("pattern", ">", "storage search pattern")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := runner.PullProfiles(storage.PullProfilesOptions{OutDir: *outDir, Pattern: *pattern}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runRestore(args []string, stdout, stderr io.Writer, runner Runner) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	srcDir := fs.String("src", "", "source directory")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	opts := storage.ProfileRestoreOptions{SourceDir: *srcDir}

	var (
		plan storage.RestorePlan
		err  error
	)
	switch args[0] {
	case "plan":
		plan, err = runner.PlanProfileRestore(opts)
	case "apply":
		plan, err = runner.ApplyProfileRestore(opts)
	default:
		usage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runScripts(args []string, stdout, stderr io.Writer, runner Runner) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("scripts start", flag.ContinueOnError)
		fs.SetOutput(stderr)
		name := fs.String("name", "", "script name")
		queryRef := fs.String("query-ref", "", "query ref")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		hash, err := runner.StartScript(*name, *queryRef)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(map[string]string{"hash": hash}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "stop":
		fs := flag.NewFlagSet("scripts stop", flag.ContinueOnError)
		fs.SetOutput(stderr)
		name := fs.String("name", "", "script name")
		queryRef := fs.String("query-ref", "", "query ref")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.StopScript(*name, *queryRef); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		usage(stderr)
		return 2
	}
}

func runStorage(args []string, stderr io.Writer, runner Runner) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "pull":
		fs := flag.NewFlagSet("storage pull", flag.ContinueOnError)
		fs.SetOutput(stderr)
		outDir := fs.String("out", "", "output directory")
		pattern := fs.String("pattern", ">", "storage search pattern")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.PullStorage(*outDir, *pattern); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "push":
		fs := flag.NewFlagSet("storage push", flag.ContinueOnError)
		fs.SetOutput(stderr)
		srcDir := fs.String("src", "", "source directory")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.PushStorage(*srcDir); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "delete":
		fs := flag.NewFlagSet("storage delete", flag.ContinueOnError)
		fs.SetOutput(stderr)
		pattern := fs.String("pattern", "", "storage search pattern")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.DeleteStorage(*pattern); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		usage(stderr)
		return 2
	}
}

type StorageRunner struct {
	store storage.Storage
	msg   messenger.Messenger
}

func NewStorageRunner(store storage.Storage, msg ...messenger.Messenger) *StorageRunner {
	runner := &StorageRunner{store: store}
	if len(msg) > 0 {
		runner.msg = msg[0]
	}
	return runner
}

func (r *StorageRunner) PullProfiles(opts storage.PullProfilesOptions) error {
	return storage.PullProfiles(r.store, opts)
}

func (r *StorageRunner) PlanProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error) {
	return storage.PlanProfileRestore(r.store, opts)
}

func (r *StorageRunner) ApplyProfileRestore(opts storage.ProfileRestoreOptions) (storage.RestorePlan, error) {
	return storage.ApplyProfileRestore(r.store, opts)
}

func (r *StorageRunner) PullStorage(outDir, pattern string) error {
	return pullWorkspace(r.store, outDir, pattern)
}

func (r *StorageRunner) PushStorage(srcDir string) error {
	return pushWorkspace(r.store, srcDir)
}

func (r *StorageRunner) DeleteStorage(pattern string) error {
	return deleteWorkspace(r.store, pattern)
}

func (r *StorageRunner) StartScript(name, queryRef string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("start script: name is required")
	}
	var resp scriptAPIResponse
	if err := r.requestScriptAPI("script.start", map[string]any{
		"name":     name,
		"queryRef": queryRef,
	}, &resp); err != nil {
		return "", err
	}
	return resp.Hash, nil
}

func (r *StorageRunner) StopScript(name, queryRef string) error {
	if name == "" {
		return fmt.Errorf("stop script: name is required")
	}
	return r.requestScriptAPI("script.stop", map[string]any{
		"name":     name,
		"queryRef": queryRef,
	}, nil)
}

func DefaultRunnerFromEnv() (Runner, error) {
	url := os.Getenv("SB_NATS_URL")
	if url == "" {
		return nil, fmt.Errorf("SB_NATS_URL is required")
	}
	store, err := storage.ConnectURL(url)
	if err != nil {
		return nil, err
	}
	msg, err := messenger.ConnectURL(url)
	if err != nil {
		return nil, err
	}
	return NewStorageRunner(store, msg), nil
}

type scriptAPIResponse struct {
	OK    bool   `json:"ok"`
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

func (r *StorageRunner) requestScriptAPI(subject string, body any, dest *scriptAPIResponse) error {
	if r.msg == nil {
		return fmt.Errorf("%s: messenger is not configured", subject)
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	respMsg, err := r.msg.Request(subject, data, 5*time.Second)
	if err != nil {
		return err
	}
	var resp scriptAPIResponse
	if err := json.Unmarshal(respMsg.Data, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if dest != nil {
		*dest = resp
	}
	return nil
}
