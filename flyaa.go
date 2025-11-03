package flyaa

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/igolaizola/flyaa/pkg/aa"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Debug       bool
	Proxy       string
	BaseURL     string
	Origin      string
	Destination string
	Date        string
	Passengers  int
	CabinClass  string
}

type response struct {
	SearchMetadata struct {
		Origin      string `json:"origin"`
		Destination string `json:"destination"`
		Date        string `json:"date"`
		Passengers  int    `json:"passengers"`
		CabinClass  string `json:"cabin_class"`
	} `json:"search_metadata"`
	Flights []aa.Flight `json:"flights"`
}

func Run(ctx context.Context, cfg *Config) error {
	// Validate input
	if cfg.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	origin := strings.ToUpper(cfg.Origin)
	if len(origin) != 3 {
		return fmt.Errorf("origin airport code must be 3 letters")
	}
	destination := strings.ToUpper(cfg.Destination)
	if len(destination) != 3 {
		return fmt.Errorf("destination airport code must be 3 letters")
	}
	date := cfg.Date
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("flight date must be in YYYY-MM-DD format: %w", err)
	}
	passengers := cfg.Passengers
	if passengers <= 0 {
		passengers = 1
	}

	// Map cabin class
	cabinClass := strings.ToLower(cfg.CabinClass)
	var productType string
	switch cabinClass {
	case "economy":
		productType = "BASIC_ECONOMY"
	case "main":
		productType = "COACH"
	case "main-plus":
		productType = "COACH_FLEXIBLE"
	default:
		return fmt.Errorf("unsupported cabin class %q, supported values are: economy, main, main-plus", cabinClass)
	}

	// Create service client
	svc, err := aa.New(&aa.Config{
		Debug:   cfg.Debug,
		Proxy:   cfg.Proxy,
		BaseURL: cfg.BaseURL,
	})
	if err != nil {
		return fmt.Errorf("couldn't create aa client: %w", err)
	}

	// Search flightsPrice
	var flightsPrice, flightPoints []aa.Flight

	// Run both searches concurrently
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(2)
	g.Go(func() error {
		// Regular search
		fs, err := svc.Search(ctx, origin, destination, date, passengers, productType, false)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		flightsPrice = fs
		return nil
	})
	g.Go(func() error {
		// Points search
		fs, err := svc.Search(ctx, origin, destination, date, passengers, productType, true)
		if err != nil {
			return fmt.Errorf("search points failed: %w", err)
		}
		flightPoints = fs
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}

	// Combine results
	var flights []aa.Flight
	lookup := make(map[string]aa.Flight)
	for _, f := range flightPoints {
		lookup[f.ID()] = f
	}
	for i := range flightsPrice {
		flight := flightsPrice[i]
		fp, ok := lookup[flight.ID()]
		if !ok {
			continue
		}
		if fp.PointsRequired == 0 {
			continue
		}
		flight.PointsRequired = fp.PointsRequired
		flight.TaxesFeesUSD = fp.TaxesFeesUSD
		flight.CPP = (flight.CashPriceUSD - fp.TaxesFeesUSD) / float64(fp.PointsRequired) * 100.0
		flight.CPP = round(flight.CPP, 2)
		flights = append(flights, flight)
	}

	// Print response
	var resp response
	resp.SearchMetadata.Origin = origin
	resp.SearchMetadata.Destination = destination
	resp.SearchMetadata.Date = date
	resp.SearchMetadata.Passengers = passengers
	resp.SearchMetadata.CabinClass = cabinClass
	resp.Flights = flights

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("couldn't marshal response: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func round(x float64, prec int) float64 {
	f := math.Pow(10, float64(prec))
	return math.Round(x*f) / f
}
