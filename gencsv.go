package main

import (
	"fmt"
	"os"
	"time"

	csvpkg "github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/csvgen"
)

// GenCSVCommand implements the gen-csv subcommand.
type GenCSVCommand struct {
	Type         string  `long:"type" required:"true" description:"CSV type slug (e.g. players, bets, bonus)"`
	Rows         int     `long:"rows" default:"100" description:"Number of data rows to generate"`
	Output       string  `long:"output" description:"Output file path (default: stdout)"`
	InjectErrors bool    `long:"inject-errors" description:"Enable error injection"`
	ErrorRate    float64 `long:"error-rate" default:"0.1" description:"Fraction of error rows (0.0-1.0)"`
	Seed         *int64  `long:"seed" description:"Random seed for deterministic output"`
}

// Execute runs the gen-csv subcommand.
func (g *GenCSVCommand) Execute(args []string) error {
	csvType, err := csvpkg.CSVTypeFromSlug(g.Type)
	if err != nil {
		return fmt.Errorf("invalid --type: %w", err)
	}

	if g.Rows < 0 {
		return fmt.Errorf("--rows must be non-negative")
	}

	if g.ErrorRate < 0 || g.ErrorRate > 1 {
		return fmt.Errorf("--error-rate must be between 0.0 and 1.0")
	}

	var opts []csvgen.Option

	// If seed was explicitly provided via --seed, use it (including 0);
	// otherwise use time-based seed for non-deterministic output.
	if g.Seed != nil {
		opts = append(opts, csvgen.WithSeed(*g.Seed))
	} else {
		opts = append(opts, csvgen.WithSeed(time.Now().UnixNano()))
	}

	if g.InjectErrors {
		opts = append(opts, csvgen.WithErrorInjection(g.ErrorRate))
	}

	data, err := csvgen.GenerateCSV(csvType, g.Rows, opts...)
	if err != nil {
		return fmt.Errorf("generating CSV: %w", err)
	}

	if g.Output != "" {
		if err := os.WriteFile(g.Output, data, 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Generated %d rows of %s CSV to %s\n", g.Rows, g.Type, g.Output)
	} else {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("writing to stdout: %w", err)
		}
	}

	return nil
}
