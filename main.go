package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/sandbox-kit-spec/v3/spec"
)

const usage = `kitgate: CI gate for Docker Sandbox Kit permission widenings (SPEC-v3 7.4)

Usage:
  kitgate surface <kit>                 print the permission surface as JSON
  kitgate lock <kit> <surface.lock>     write the surface to a lock file
  kitgate check <kit> <surface.lock>    exit 1 if kit widens the locked surface
  kitgate diff <old> <new>              exit 1 if new widens old

<kit>, <old>, <new> may be:
  - a Kit descriptor YAML (authoring form)
  - published descriptor JSON (annotation value)
  - an OCI image manifest JSON with vnd.docker.sandbox.kit.descriptor
  - a directory containing manifest.json or index.json

Flags:
  --ignore-deny-removed   do not treat a removed deny rule as a widening
                          (breaks the SPEC-v3 7.4 rule; used by demo.sh --break)

Exit codes: 0 ok, 1 widening, 2 usage or input error
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	ignoreDenyRemoved := false
	var positional []string
	for _, a := range args {
		switch a {
		case "-h", "--help", "help":
			fmt.Fprint(os.Stdout, usage)
			return 0
		case "--ignore-deny-removed":
			ignoreDenyRemoved = true
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	cmd := positional[0]
	rest := positional[1:]
	switch cmd {
	case "surface":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: kitgate surface <kit>")
			return 2
		}
		d, src, err := loadDescriptor(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		s := spec.SurfaceOf(d)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(s); err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		fmt.Fprintf(os.Stderr, "kitgate: loaded %s from %s\n", src, rest[0])
		return 0

	case "lock":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: kitgate lock <kit> <surface.lock>")
			return 2
		}
		d, _, err := loadDescriptor(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		s := spec.SurfaceOf(d)
		raw, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(rest[1], raw, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		fmt.Printf("wrote %s (%d bytes)\n", rest[1], len(raw))
		return 0

	case "check":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: kitgate check <kit> <surface.lock>")
			return 2
		}
		d, _, err := loadDescriptor(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		granted, err := loadSurfaceLock(rest[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: %v\n", err)
			return 2
		}
		return reportWidenings(granted, spec.SurfaceOf(d), ignoreDenyRemoved)

	case "diff":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: kitgate diff <old> <new>")
			return 2
		}
		oldD, oldSrc, err := loadDescriptor(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: old: %v\n", err)
			return 2
		}
		newD, newSrc, err := loadDescriptor(rest[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "kitgate: new: %v\n", err)
			return 2
		}
		fmt.Fprintf(os.Stderr, "kitgate: old=%s (%s) new=%s (%s)\n", rest[0], oldSrc, rest[1], newSrc)
		return reportWidenings(spec.SurfaceOf(oldD), spec.SurfaceOf(newD), ignoreDenyRemoved)

	default:
		fmt.Fprintf(os.Stderr, "kitgate: unknown command %q\n%s", cmd, usage)
		return 2
	}
}

func loadSurfaceLock(path string) (spec.Surface, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return spec.Surface{}, err
	}
	var s spec.Surface
	if err := json.Unmarshal(raw, &s); err != nil {
		return spec.Surface{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func reportWidenings(granted, candidate spec.Surface, ignoreDenyRemoved bool) int {
	ws := spec.DiffWidenings(granted, candidate)
	if ignoreDenyRemoved {
		filtered := ws[:0]
		for _, w := range ws {
			if strings.Contains(w.Category, "deny-removed") {
				continue
			}
			filtered = append(filtered, w)
		}
		ws = filtered
	}
	if len(ws) == 0 {
		fmt.Println("0 widenings")
		return 0
	}
	fmt.Printf("%d widening(s):\n", len(ws))
	fmt.Println(spec.FormatWidenings(ws))
	return 1
}
