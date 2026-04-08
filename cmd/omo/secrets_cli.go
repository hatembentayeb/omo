package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"omo/pkg/secrets"
)

const secretsCLIUsage = `omo secrets – CRUD operations on the KeePass database (~/.omo/secrets/omo.kdbx)

Paths follow the convention:  pluginName/environment/entryName
  e.g.  redis/production/cache   or   github/default/mytoken

Usage:
  omo secrets list   [prefix]
  omo secrets get    <path>
  omo secrets put    <path>  [flags]
  omo secrets delete <path>

Commands:
  list    List all entry paths, optionally filtered by prefix
  get     Print all fields of an entry as JSON
  put     Create or update an entry (only supplied flags are written)
  delete  Remove an entry

Flags for 'put':
  --username  string
  --password  string
  --url       string
  --notes     string
  --attr      key=value   (repeatable – custom attributes)

Examples:
  omo secrets list
  omo secrets list redis
  omo secrets get  redis/production/cache
  omo secrets put  redis/production/cache --username admin --password s3cr3t --url redis://localhost:6379
  omo secrets put  redis/production/cache --attr tls_cert="-----BEGIN CERT-----..."
  omo secrets delete redis/production/cache
`

// runSecretsCLI is the entrypoint for the `omo secrets` subcommand.
// It does NOT start the TUI – it reads/writes the KeePass DB and exits.
func runSecretsCLI(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, secretsCLIUsage)
		os.Exit(1)
	}

	p, err := secrets.New()
	if err != nil {
		fatalf("open secrets database: %v", err)
	}
	defer p.Close()

	cmd, rest := args[0], args[1:]

	switch cmd {
	case "list":
		runSecretsListCmd(p, rest)
	case "get":
		runSecretsGetCmd(p, rest)
	case "put":
		runSecretsPutCmd(p, rest)
	case "delete", "rm":
		runSecretsDeleteCmd(p, rest)
	case "help", "--help", "-h":
		fmt.Print(secretsCLIUsage)
	default:
		fmt.Fprintf(os.Stderr, "omo secrets: unknown command %q\n\n%s", cmd, secretsCLIUsage)
		os.Exit(1)
	}
}

// ── list ─────────────────────────────────────────────────────────────────────

func runSecretsListCmd(p secrets.Provider, args []string) {
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	paths, err := p.List(prefix)
	if err != nil {
		fatalf("list: %v", err)
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "(no entries found)")
		return
	}
	for _, path := range paths {
		fmt.Println(path)
	}
}

// ── get ──────────────────────────────────────────────────────────────────────

func runSecretsGetCmd(p secrets.Provider, args []string) {
	if len(args) == 0 {
		fatalf("get: path argument required\nUsage: omo secrets get <pluginName/environment/entryName>")
	}

	entry, err := p.Get(args[0])
	if err != nil {
		fatalf("get %s: %v", args[0], err)
	}

	out := map[string]interface{}{
		"path":     args[0],
		"username": entry.UserName,
		"password": entry.Password,
		"url":      entry.URL,
		"notes":    entry.Notes,
	}
	if len(entry.CustomAttributes) > 0 {
		out["attributes"] = entry.CustomAttributes
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalf("encode output: %v", err)
	}
}

// ── put ──────────────────────────────────────────────────────────────────────

// attrList is a flag.Value that collects repeatable --attr key=value pairs.
type attrList []string

func (a *attrList) String() string  { return strings.Join(*a, ", ") }
func (a *attrList) Set(v string) error {
	if !strings.Contains(v, "=") {
		return fmt.Errorf("--attr must be key=value, got %q", v)
	}
	*a = append(*a, v)
	return nil
}

func runSecretsPutCmd(p secrets.Provider, args []string) {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	username := fs.String("username", "", "username field")
	password := fs.String("password", "", "password field")
	url := fs.String("url", "", "url field")
	notes := fs.String("notes", "", "notes field")
	var attrs attrList
	fs.Var(&attrs, "attr", "custom attribute in key=value format (repeatable)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: omo secrets put <path> [--username X] [--password Y] [--url Z] [--notes N] [--attr key=value ...]\n")
	}

	if len(args) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	path := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(1)
	}

	// Load existing entry so we do a partial update (only flags provided are overwritten).
	existing, err := p.Get(path)
	if err != nil {
		// Entry doesn't exist yet – start from blank.
		existing = &secrets.Entry{CustomAttributes: map[string]string{}}
	}
	if existing.CustomAttributes == nil {
		existing.CustomAttributes = map[string]string{}
	}

	if *username != "" {
		existing.UserName = *username
	}
	if *password != "" {
		existing.Password = *password
	}
	if *url != "" {
		existing.URL = *url
	}
	if *notes != "" {
		existing.Notes = *notes
	}
	for _, kv := range attrs {
		idx := strings.Index(kv, "=")
		existing.CustomAttributes[kv[:idx]] = kv[idx+1:]
	}

	if err := p.Put(path, existing); err != nil {
		fatalf("put %s: %v", path, err)
	}
	fmt.Printf("saved: %s\n", path)
}

// ── delete ───────────────────────────────────────────────────────────────────

func runSecretsDeleteCmd(p secrets.Provider, args []string) {
	if len(args) == 0 {
		fatalf("delete: path argument required\nUsage: omo secrets delete <pluginName/environment/entryName>")
	}

	path := args[0]

	// Confirm unless --force is passed.
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(1)
	}

	if !*force {
		fmt.Printf("delete %q? [y/N] ", path)
		var answer string
		fmt.Scanln(&answer) //nolint:errcheck
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("aborted")
			return
		}
	}

	if err := p.Delete(path); err != nil {
		fatalf("delete %s: %v", path, err)
	}
	fmt.Printf("deleted: %s\n", path)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func fatalf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "omo secrets: "+format+"\n", a...)
	os.Exit(1)
}
