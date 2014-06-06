package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"

	forecast "github.com/mlbright/forecast/v2"
)

const (
	// geocodeHost is the host at which we access the geocoding API.
	// TODO(alex): allow option for HTTPS with key set up.
	geocodeHost = "maps.googleapis.com/"
	// geocodePath is the path at which we access the geocoding API.
	geocodePath = "maps/api/geocode/json"

	// forecastIoEnvKey is the environment variable key that should be set with
	// the forecastIo API key.
	forecastIoEnvKey = "FORECAST_IO_API_KEY"

	// goforecastState is the name of the file that contains the forecast io
	// key.
	goforecastState = ".goforecast"
)

// stateFilePath is where the goforecastState is located.
var stateFilePath = os.Getenv("HOME")

func main() {
	app := setupCliApp()

	if err := app.Run(os.Args); err != nil {
		printError(err)
	}
}

// printError handles the exiting of the program and displaying all errors. No-
// op if error is nil.
func printError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}

// setupCliApp initializes a new *cli.App and populates its fields and flags.
func setupCliApp() *cli.App {
	app := cli.NewApp()
	app.Name = "goforecast"
	app.Usage = `goforecast looks up three days of weather based upon a partial
	    address; e.g., a zip code.`
	app.Author = "Alex Toombs"

	populateCommands(app)
	return app
}

// populateCommands sets up all commands on the new command line application.
func populateCommands(app *cli.App) {
	app.Commands = []cli.Command{
		cli.Command{
			Name:        "lookup",
			ShortName:   "l",
			Description: "`lookup` looks up weather for a partial or whole address.",
			Usage:       "lookup \"[address]...\"",
			Action: func(c *cli.Context) {
				if len(c.Args()) == 0 {
					printError(fmt.Errorf("missing address"))
				}

				addr := c.Args().First()
				u, err := buildGeocodingURL("http", parseGeocodingAddr(addr))
				if err != nil {
					printError(err)
				}

				geoResp, err := getGeocodingLocation(http.DefaultClient, u)
				if err != nil {
					printError(err)
				}

				fc, err := getForecast(geoResp.Results[0].Geometry.Location)
				if err != nil {
					printError(err)
				}

				renderForecast(fc, addr)
			},
		},
	}
}

// buildGeocodingURL builds a parsed URL with query values passed in.
func buildGeocodingURL(scheme string, vals url.Values) (*url.URL, error) {
	u := &url.URL{
		Scheme: scheme,
		Host:   geocodeHost,
		Path:   geocodePath,
	}
	if len(vals) != 0 {
		u.RawQuery = vals.Encode()

	}
	return url.Parse(u.String())
}

// parseGeocodingAddr parses a string address into url query values.
func parseGeocodingAddr(addr string) url.Values {
	vals := url.Values{}
	vals.Add("address", url.QueryEscape(addr))
	vals.Add("sensor", "false")
	return vals
}

// getGeocodingLocation attempts to get a valid geocoding response back for the
// entered address.
func getGeocodingLocation(client *http.Client, u *url.URL) (*geocodingResponse, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	errStr := "on request: got code %d"
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusCreated:
	default:
		return nil, fmt.Errorf(errStr, resp.StatusCode)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	geoResp := new(geocodingResponse)
	if err := json.Unmarshal(b, &geoResp); err != nil {
		return nil, err
	}

	if len(geoResp.Results) == 0 {
		return nil, fmt.Errorf("no geocoding results returned")
	}
	return geoResp, nil
}

// geocodingResponse is a golang struct that some fields of a geocoding response
// will cleanly unmarshal onto.
type geocodingResponse struct {
	Results []geocodingResult `json:"results"`
}

// geocodingResult is one result from the geocoding lookup.
type geocodingResult struct {
	Geometry *geometry `json:"geometry"`
}

// geometry embeds a ocation.
type geometry struct {
	Location *location `json:"location"`
}

// location is a latitude/longitutde pair.
type location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// getForecast gets the forecast from forecast.io.
func getForecast(loc *location) (*forecast.Forecast, error) {
	key, err := getForecastIOKey()
	if err != nil {
		return nil, err
	}

	// Record state.
	if err := dumpState(&forecastIoState{
		ApiKey: key,
	}); err != nil {
		return nil, err
	}

	// TODO(alex): allow for multiple time windows and units.
	return forecast.Get(key,
		fmt.Sprintf("%.2f", loc.Lat),
		fmt.Sprintf("%.2f", loc.Lng),
		"now",
		forecast.US,
	)
}

// renderForecast nicely displays the forecast.
// TODO(alex): more information rendered!
// TODO(alex): rendered units should be dynamic.
func renderForecast(fc *forecast.Forecast, addr string) {
	fmt.Printf("Displaying current forecast for %s\n\n", addr)

	fmt.Println("---Currently---")
	fmt.Printf("Summary: %s\n\n", fc.Currently.Summary)
	fmt.Printf("Temperature: %.2f F\n", fc.Currently.Temperature)
	fmt.Printf("Pressure: %.2f kPa\n", fc.Currently.Pressure)
	fmt.Printf("Wind Speed: %.2f mph\n", fc.Currently.WindSpeed)
	fmt.Printf("Precipitation Chance: %.2f%%\n", fc.Currently.PrecipProbability)
}

// forecastIoState is a json struct that we read from/write to disk to keep
// state.
type forecastIoState struct {
	ApiKey string `json:"api_key"`
}

// getForecastIoKey looks up the Forecast IO API key from disk or the user's
// environment.
func getForecastIOKey() (string, error) {
	state, err := restoreState()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if state != nil && state.ApiKey != "" {
		return state.ApiKey, nil
	}

	// Try environment.
	k := os.Getenv(forecastIoEnvKey)
	if k == "" {
		return "", fmt.Errorf("could not find Forecast IO API key. Please set with \"export %s=<key>\"",
			forecastIoEnvKey)
	}
	return k, nil
}

// restoreState restores state from disk.
func restoreState() (*forecastIoState, error) {
	f, err := os.Open(filepath.Join(stateFilePath, goforecastState))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var state *forecastIoState
	if err := json.Unmarshal(b, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// dumpState writes state to disk.
func dumpState(state *forecastIoState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(stateFilePath, goforecastState), b, 0644); err != nil {
		return err
	}
	return nil
}
