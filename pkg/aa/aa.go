package aa

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type searchRequest struct {
	EnhancedSearch bool `json:"enhancedSearch"`
	Metadata       struct {
		SelectedProducts []string `json:"selectedProducts"`
		TripType         string   `json:"tripType"`
	} `json:"metadata"`
	Passengers  []searchPassenger `json:"passengers"`
	QueryParams struct {
		SliceIndex  int    `json:"sliceIndex"`
		SessionID   string `json:"sessionId"`
		SolutionSet string `json:"solutionSet"`
		SolutionID  string `json:"solutionId"`
	} `json:"queryParams"`
	RequestHeader struct {
		ClientID         string `json:"clientId"`
		TransactionID    string `json:"transactionID"`
		BookingSessionID string `json:"bookingSessionID"`
	} `json:"requestHeader"`
	Slices      []searchSlice `json:"slices"`
	TripOptions struct {
		CorporateBooking bool   `json:"corporateBooking"`
		FareType         string `json:"fareType"`
		Locale           string `json:"locale"`
		SearchType       string `json:"searchType"`
	} `json:"tripOptions"`
}

type searchPassenger struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type searchSlice struct {
	AllCarriers           bool   `json:"allCarriers"`
	Cabin                 string `json:"cabin"`
	DepartureDate         string `json:"departureDate"` // keep as string (e.g., "2025-12-15")
	Destination           string `json:"destination"`
	IncludeNearbyAirports bool   `json:"includeNearbyAirports"`
	Origin                string `json:"origin"`
}

type searchResponse struct {
	ResponseMetadata struct {
		SessionID   string `json:"sessionId"`
		SolutionSet string `json:"solutionSet"`
	} `json:"responseMetadata"`
	Error  any             `json:"error"`
	Slices []responseSlice `json:"slices"`
}

type responseSlice struct {
	Segments          []segmentResponse       `json:"segments"`
	CheapestPrice     responsePricingDetail   `json:"cheapestPrice"`
	PricingDetail     []responsePricingDetail `json:"pricingDetail"`
	DepartureDateTime string                  `json:"departureDateTime"`
	ArrivalDateTime   string                  `json:"arrivalDateTime"`
	Stops             int                     `json:"stops"`
	DurationInMinutes int                     `json:"durationInMinutes"`
}

type segmentResponse struct {
	Flight struct {
		CarrierCode  string `json:"carrierCode"`
		CarrierName  string `json:"carrierName"`
		FlightNumber string `json:"flightNumber"`
	} `json:"flight"`
	DepartureDateTime string           `json:"departureDateTime"`
	ArrivalDateTime   string           `json:"arrivalDateTime"`
	Destination       responseLocation `json:"destination"`
	Origin            responseLocation `json:"origin"`
}

type responseLocation struct {
	City     string `json:"city"`
	Name     string `json:"name"`
	CityName string `json:"cityName"`
	Code     string `json:"code"`
}

type responsePricingDetail struct {
	PerPassengerPrice        string `json:"perPassengerPrice"`
	AllPassengerTaxesAndFees struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"allPassengerTaxesAndFees"`
	ProductType string `json:"productType"`
	SolutionID  string `json:"solutionID"`
}

type Flight struct {
	IsNonstop      bool            `json:"is_nonstop"`
	Segments       []FlightSegment `json:"segments"`
	TotalDuration  string          `json:"total_duration"`
	PointsRequired int             `json:"points_required"`
	CashPriceUSD   float64         `json:"cash_price_usd"`
	TaxesFeesUSD   float64         `json:"taxes_fees_usd"`
	CPP            float64         `json:"cpp"`
}

type FlightSegment struct {
	FlightNumber  string `json:"flight_number"`
	DepartureTime string `json:"departure_time"`
	ArrivalTime   string `json:"arrival_time"`
}

// ID generates a unique ID for the flight based on its segments' flight numbers.
func (f *Flight) ID() string {
	var numbers []string
	for _, seg := range f.Segments {
		numbers = append(numbers, seg.FlightNumber)
	}
	return strings.Join(numbers, "_")
}

func (c *Client) Search(ctx context.Context, origin, destination, date string, passengers int, productType string, redeemPoints bool) ([]Flight, error) {
	// Generate random IDs
	transactionID := uuid.New().String()
	bookingSessionID := uuid.New().String()

	// Create request
	var req searchRequest
	req.Metadata.SelectedProducts = []string{}
	req.Metadata.TripType = "oneWay"
	req.Passengers = []searchPassenger{
		{Type: "adult", Count: passengers},
	}
	req.RequestHeader.ClientID = "mobile"
	req.RequestHeader.TransactionID = transactionID
	req.RequestHeader.BookingSessionID = bookingSessionID
	req.Slices = []searchSlice{
		{
			AllCarriers:           true,
			Cabin:                 "",
			DepartureDate:         date,
			Destination:           destination,
			IncludeNearbyAirports: false,
			Origin:                origin,
		},
	}
	req.TripOptions.FareType = "Lowest"
	req.TripOptions.Locale = "en_US"
	req.TripOptions.SearchType = "revenue"
	if redeemPoints {
		req.TripOptions.SearchType = "award"
	}

	// Do request
	var resp searchResponse
	if _, err := c.do(ctx, "POST", "search/itinerary/v2.0", &req, &resp); err != nil {
		return nil, err
	}

	// Parse response
	var flights []Flight
	for _, slice := range resp.Slices {
		var segs []FlightSegment
		for _, sg := range slice.Segments {
			// Build flight number
			flightNumber := fmt.Sprintf("%s%s", sg.Flight.CarrierCode, sg.Flight.FlightNumber)

			// Parse times
			departureTime, err := parseTime(sg.DepartureDateTime)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse departure time: %w", err)
			}
			arrivalTime, err := parseTime(sg.ArrivalDateTime)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse arrival time: %w", err)
			}
			segs = append(segs, FlightSegment{
				FlightNumber:  flightNumber,
				DepartureTime: departureTime,
				ArrivalTime:   arrivalTime,
			})
		}

		// Find pricing
		var cashPrice float64
		var pointsRequired int
		var taxesFees float64
		if redeemPoints {
			// For points searches, use always the cheapest price
			var err error
			pointsRequired, err = parseAbbrevInt(slice.CheapestPrice.PerPassengerPrice)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse points price %q: %w", slice.CheapestPrice.PerPassengerPrice, err)
			}
			taxesFees = slice.CheapestPrice.AllPassengerTaxesAndFees.Amount / float64(passengers)
		} else {
			// For cash searches, find the matching product type
			for _, pd := range slice.PricingDetail {
				if pd.ProductType != productType {
					continue
				}
				cashPrice = pd.AllPassengerTaxesAndFees.Amount / float64(passengers)
			}
			if cashPrice == 0 {
				// No matching cabin class found
				continue
			}
		}

		// Format duration
		hours := slice.DurationInMinutes / 60
		minutes := slice.DurationInMinutes % 60
		duration := fmt.Sprintf("%dh %dm", hours, minutes)

		// Append flight
		flights = append(flights, Flight{
			IsNonstop:      slice.Stops == 0,
			TotalDuration:  duration,
			Segments:       segs,
			CashPriceUSD:   cashPrice,
			PointsRequired: pointsRequired,
			TaxesFeesUSD:   taxesFees,
		})
	}
	return flights, nil
}

// parseAbbrevInt converts strings like "12.5K", "4K", "300" into an int.
// Supported suffixes: K (thousand), M (million), B (billion). Case-insensitive.
// It ignores spaces and commas. Returns an error on invalid input.
func parseAbbrevInt(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty input")
	}
	// Normalize: trim, remove spaces and commas
	in := strings.TrimSpace(s)
	in = strings.ReplaceAll(in, ",", "")
	in = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1 // drop spaces
		}
		return r
	}, in)
	if in == "" {
		return 0, errors.New("no digits")
	}

	// Check last rune for unit
	last := in[len(in)-1]
	mult := 1.0
	switch last {
	case 'k', 'K':
		mult = 1e3
		in = in[:len(in)-1]
	case 'm', 'M':
		mult = 1e6
		in = in[:len(in)-1]
	case 'b', 'B':
		mult = 1e9
		in = in[:len(in)-1]
	}

	// Allow leading +/-, integers or floats
	if in == "" || in == "+" || in == "-" {
		return 0, errors.New("invalid number")
	}

	// Try integer fast-path
	if !strings.ContainsAny(in, ".eE") {
		n, err := strconv.ParseInt(in, 10, 64)
		if err != nil {
			return 0, err
		}
		return int(math.Round(float64(n) * mult)), nil
	}

	// Float path (for things like "12.5K")
	f, err := strconv.ParseFloat(in, 64)
	if err != nil {
		return 0, err
	}
	return int(math.Round(f * mult)), nil
}

// parseTime parses a time string in RFC3339Nano format and returns it in "15:04" format.
func parseTime(s string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return "", fmt.Errorf("couldn't parse time %q: %w", s, err)
	}
	return t.Format("15:04"), nil
}
