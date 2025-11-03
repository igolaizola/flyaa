package cli

import (
	"context"
	"flag"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/igolaizola/flyaa"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/peterbourgon/ff/v3/ffyaml"
)

func NewCommand(version, commit, date string) *ffcli.Command {
	fs := flag.NewFlagSet("flyaa", flag.ExitOnError)

	_ = fs.String("config", "", "config file (optional)")
	var cfg flyaa.Config

	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	fs.StringVar(&cfg.Proxy, "proxy", "", "proxy URL")
	fs.StringVar(&cfg.BaseURL, "base-url", "", "AA API base URL")
	fs.StringVar(&cfg.Origin, "origin", "LAX", "origin airport code")
	fs.StringVar(&cfg.Destination, "destination", "JFK", "destination airport code")
	fs.StringVar(&cfg.Date, "date", "2025-12-15", "flight date (YYYY-MM-DD)")
	fs.IntVar(&cfg.Passengers, "passengers", 1, "number of passengers")
	fs.StringVar(&cfg.CabinClass, "cabin-class", "main", "cabin class (economy, main, main-plus)")

	return &ffcli.Command{
		ShortUsage: "flyaa [flags] <subcommand>",
		FlagSet:    fs,
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ffyaml.Parser),
			ff.WithEnvVarPrefix("FLYAA"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flyaa.Run(ctx, &cfg)
		},
		Subcommands: []*ffcli.Command{
			newVersionCommand(version, commit, date),
		},
	}
}

func newVersionCommand(version, commit, date string) *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "flyaa version",
		ShortHelp:  "print version",
		Exec: func(ctx context.Context, args []string) error {
			v := version
			if v == "" {
				if buildInfo, ok := debug.ReadBuildInfo(); ok {
					v = buildInfo.Main.Version
				}
			}
			if v == "" {
				v = "dev"
			}
			versionFields := []string{v}
			if commit != "" {
				versionFields = append(versionFields, commit)
			}
			if date != "" {
				versionFields = append(versionFields, date)
			}
			fmt.Println(strings.Join(versionFields, " "))
			return nil
		},
	}
}
