// Hivebook CI — the project's checks as code, run identically locally and in CI.
//
// `dagger call check` is the single enforcement entrypoint: it runs the proto
// checks (buf lint, format, gen-drift), the web checks (prettier, eslint, tsc),
// and the API checks (build, vet, test) in parallel, each in a clean, pinned
// container. See docs: standards/ci.adoc.
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"dagger/hivebook/internal/dagger"
)

// Toolchain images are pinned to match the service Dockerfiles, so the checks
// and the shipped images agree on versions.
const (
	nodeImage   = "node:22-alpine"      // web/Dockerfile
	goImage     = "golang:1.26-alpine"  // api/Dockerfile
	dindImage   = "docker:27-dind"      // engine for testcontainers
	bufImage    = "bufbuild/buf:1.70.0" // justfile `proto` recipe
	alpineImage = "alpine:3.20"         // tiny base for the gen-drift diff
	pnpmVer     = "10"                  // web/Dockerfile pins pnpm@10
)

type Hivebook struct {
	// The repository root.
	Source *dagger.Directory
}

func New(
	// The repository root; defaults to where Dagger is invoked from. Heavy and
	// generated paths are excluded so the context stays small and reproducible.
	// +defaultPath="/"
	// +ignore=["**/node_modules", "**/.next", "**/dist", ".git", ".dagger"]
	source *dagger.Directory,
) *Hivebook {
	return &Hivebook{Source: source}
}

// Check runs every enforcement check (proto + web + api) in parallel and reports
// the result. Errors if any check fails. The CI and `just check` entrypoint.
func (m *Hivebook) Check(ctx context.Context) (string, error) {
	type result struct {
		name string
		msg  string
		err  error
	}
	checks := []struct {
		name string
		run  func(context.Context) (string, error)
	}{
		{"proto", m.CheckProto},
		{"web", m.CheckWeb},
		{"api", m.CheckApi},
	}

	results := make([]result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func(i int, name string, run func(context.Context) (string, error)) {
			defer wg.Done()
			msg, err := run(ctx)
			results[i] = result{name, msg, err}
		}(i, c.name, c.run)
	}
	wg.Wait()

	var report strings.Builder
	var failed []string
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, r.name)
			fmt.Fprintf(&report, "✗ %s\n%v\n", r.name, r.err)
			continue
		}
		fmt.Fprintf(&report, "✓ %s — %s\n", r.name, r.msg)
	}
	if len(failed) > 0 {
		return report.String(), fmt.Errorf("checks failed: %s", strings.Join(failed, ", "))
	}
	return report.String(), nil
}

// CheckProto runs the contract gate: buf lint, buf format, and a generated-code
// drift check (regenerate from the proto and diff against the committed gen/, so
// a stale commit fails). Breaking-change detection needs git history and runs in
// GitHub Actions, not in this hermetic context — see standards/ci.adoc.
func (m *Hivebook) CheckProto(ctx context.Context) (string, error) {
	buf := dag.Container().
		From(bufImage).
		WithDirectory("/work", m.Source).
		WithWorkdir("/work")

	if _, err := buf.
		WithExec([]string{"buf", "lint"}).
		WithExec([]string{"buf", "format", "--diff", "--exit-code"}).
		Sync(ctx); err != nil {
		return "", err
	}

	// Regenerate with the same two templates `just proto` uses, then diff the
	// fresh output against what's committed.
	regen := buf.
		WithExec([]string{"buf", "generate", "--template", "buf.gen.go.yaml"}).
		WithExec([]string{"buf", "generate", "--template", "buf.gen.es.yaml", "--include-imports"})

	_, err := dag.Container().
		From(alpineImage).
		WithDirectory("/committed/go", m.Source.Directory("api/internal/gen")).
		WithDirectory("/committed/es", m.Source.Directory("web/lib/gen")).
		WithDirectory("/fresh/go", regen.Directory("/work/api/internal/gen")).
		WithDirectory("/fresh/es", regen.Directory("/work/web/lib/gen")).
		WithExec([]string{"sh", "-c", "diff -r /committed/go /fresh/go && diff -r /committed/es /fresh/es"}).
		Sync(ctx)
	if err != nil {
		return "", fmt.Errorf("generated code is stale — run `just proto` and commit: %w", err)
	}
	return "buf lint + format + gen-drift", nil
}

// CheckWeb runs the web quality gate: prettier --check, eslint, then tsc.
func (m *Hivebook) CheckWeb(ctx context.Context) (string, error) {
	_, err := m.webBase().
		WithExec([]string{"pnpm", "run", "format:check"}).
		WithExec([]string{"pnpm", "run", "lint"}).
		WithExec([]string{"pnpm", "run", "typecheck"}).
		Sync(ctx)
	if err != nil {
		return "", err
	}
	return "prettier + eslint + tsc", nil
}

// CheckApi runs the API quality gate: go build, go vet, then the full test suite.
// The tests use testcontainers, so a Docker-in-Docker engine is bound for them.
func (m *Hivebook) CheckApi(ctx context.Context) (string, error) {
	if _, err := m.goBase().
		WithExec([]string{"go", "build", "./..."}).
		WithExec([]string{"go", "vet", "./..."}).
		Sync(ctx); err != nil {
		return "", err
	}

	if _, err := m.goBase().
		WithServiceBinding("docker", m.dockerEngine()).
		WithEnvVariable("DOCKER_HOST", "tcp://docker:2375").
		WithEnvVariable("TESTCONTAINERS_RYUK_DISABLED", "true").
		WithEnvVariable("TESTCONTAINERS_HOST_OVERRIDE", "docker").
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx); err != nil {
		return "", err
	}
	return "build + vet + test", nil
}

// Format rewrites the web tree with Prettier and returns the formatted web
// source (without node_modules). Apply with:  dagger call format export --path=web
func (m *Hivebook) Format() *dagger.Directory {
	return m.webBase().
		WithExec([]string{"pnpm", "run", "format"}).
		Directory("/src").
		WithoutDirectory("node_modules")
}

// webBase is a node container with the web app's dependencies installed.
func (m *Hivebook) webBase() *dagger.Container {
	return dag.Container().
		From(nodeImage).
		WithExec([]string{"npm", "install", "-g", "pnpm@" + pnpmVer}).
		WithMountedCache("/pnpm-store", dag.CacheVolume("hivebook-pnpm")).
		WithExec([]string{"pnpm", "config", "set", "store-dir", "/pnpm-store"}).
		WithDirectory("/src", m.Source.Directory("web")).
		WithWorkdir("/src").
		WithExec([]string{"pnpm", "install", "--frozen-lockfile"})
}

// goBase is a Go container with the API source, module/build caches, and CGO off
// (matching api/Dockerfile, so alpine needs no C toolchain).
func (m *Hivebook) goBase() *dagger.Container {
	return dag.Container().
		From(goImage).
		WithEnvVariable("CGO_ENABLED", "0").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("hivebook-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("hivebook-go-build")).
		WithDirectory("/src", m.Source.Directory("api")).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"})
}

// dockerEngine is a Docker-in-Docker service the API tests reach via
// testcontainers (DOCKER_HOST). Insecure TCP is fine — it is reachable only
// over the in-pipeline service binding, never published.
func (m *Hivebook) dockerEngine() *dagger.Service {
	return dag.Container().
		From(dindImage).
		WithMountedCache("/var/lib/docker", dag.CacheVolume("hivebook-dind"),
			dagger.ContainerWithMountedCacheOpts{Sharing: dagger.CacheSharingModePrivate}).
		WithEnvVariable("DOCKER_TLS_CERTDIR", "").
		WithExposedPort(2375).
		AsService(dagger.ContainerAsServiceOpts{
			UseEntrypoint:            true,
			InsecureRootCapabilities: true,
		})
}
