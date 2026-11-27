goheresy
========

`goheresy` applies `goimports`, then puts eligible three-line blocks on
one line.

    if bytes.Equal(src, formatted) {
        return nil
    }

becomes

    if bytes.Equal(src, formatted) { return nil }

Installation
------------

Requires Go 1.24 or later.

| Mechanism | Command                                                       |
|-----------|---------------------------------------------------------------|
| CLI       | `go install github.com/kerscher/goheresy/cmd/goheresy@latest` |
| Source    | `make install`                                                |
| Container | `make image`                                                  |

Formatting rules
----------------

An `if`, `else`, `for`, `range` or standalone block is compacted if
`goimports` places its opening brace and any header, one non-empty
statement, and its closing brace on three consecutive lines. Comments in
the header or block prevent compaction. A comment after the closing
brace is preserved. The compact statement has no trailing semicolon.

Compaction is limited to 120 Unicode code points per resulting line,
including indentation and any trailing comment. A tab counts as one code
point.

Function bodies, switch and select clauses, type declarations and
composite literals retain the original layout. Original fragment
formatting can change indentation inside multiline raw strings on
repeated runs.

Why!?
-----

So this:

    func loadConfig(path string) (Config, error) {
        path, err := filepath.Abs(path)
        if err != nil {
            return Config{}, fmt.Errorf("resolve config path: %w", err)
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return Config{}, fmt.Errorf("read config: %w", err)
        }
        var cfg Config
        if err := json.Unmarshal(data, &cfg); err != nil {
            return Config{}, fmt.Errorf("decode config: %w", err)
        }
        cfg.SetDefaults()
        if err := cfg.ApplyEnvironment(os.LookupEnv); err != nil {
            return Config{}, fmt.Errorf("apply environment: %w", err)
        }
        baseDir := filepath.Dir(path)
        if err := cfg.ResolvePaths(baseDir); err != nil {
            return Config{}, fmt.Errorf("resolve relative paths: %w", err)
        }
        if err := cfg.Validate(); err != nil {
            return Config{}, fmt.Errorf("validate config: %w", err)
        }
        return cfg, nil
    }

Becomes this:

    func loadConfig(path string) (Config, error) {
        path, err := filepath.Abs(path)
        if err != nil { return Config{}, fmt.Errorf("resolve config path: %w", err) }
        data, err := os.ReadFile(path)
        if err != nil { return Config{}, fmt.Errorf("read config: %w", err) }
        var cfg Config
        if err := json.Unmarshal(data, &cfg); err != nil { return Config{}, fmt.Errorf("decode config: %w", err) }
        cfg.SetDefaults()
        if err := cfg.ApplyEnvironment(os.LookupEnv); err != nil { return Config{}, fmt.Errorf("apply environment: %w", err) }
        baseDir := filepath.Dir(path)
        if err := cfg.ResolvePaths(baseDir); err != nil { return Config{}, fmt.Errorf("resolve relative paths: %w", err) }
        if err := cfg.Validate(); err != nil { return Config{}, fmt.Errorf("validate config: %w", err) }
        return cfg, nil
    }

If you don’t find this an improvement, don’t use it!
