package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	csvutil "github.com/jszwec/csvutil"
	trainmapdb "github.com/rom-vtn/trainmap-db"
)

var routeTypePrefixMapping = map[trainmapdb.RouteType]string{
	trainmapdb.RouteTypeTram:      "t",
	trainmapdb.RouteTypeHeavyRail: "r",
	trainmapdb.RouteTypeBus:       "b",
}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s config_file.json output_name.zip", os.Args[0])
	}

	configFileName := os.Args[1]
	outputFileName := os.Args[2]

	configContents, err := os.ReadFile(configFileName)
	if err != nil {
		panic(err)
	}

	var config trainmapdb.LoaderConfig
	err = json.Unmarshal(configContents, &config)
	if err != nil {
		panic(err)
	}

	f, err := trainmapdb.NewFetcher(config.DatabasePath, nil)
	if err != nil {
		panic(err)
	}

	err = f.LoadDatabase(config)
	if err != nil {
		panic(err)
	}

	outputWriter, err := os.Create(outputFileName)
	if err != nil {
		panic(err)
	}

	err = rewriteAsUnfucked(f, outputWriter)
	if err != nil {
		panic(err)
	}
}

type routeUsageStatus struct {
	route                         trainmapdb.Route
	hasTram, hasHeavyRail, hasBus bool
}

type feededRouteId struct {
	routeId, feedId string
}

func rewriteAsUnfucked(f *trainmapdb.Fetcher, outputFile io.Writer) error {
	zipWriter := zip.NewWriter(outputFile)
	defer zipWriter.Close()

	usageMap := make(map[feededRouteId]routeUsageStatus)
	//unfuck trips (trip short name vs headsign nyaaaaaa)
	allTrips, err := unfuckTrips(f, zipWriter, &usageMap)
	if err != nil {
		return err
	}
	//unfuck route types
	//use the filled usage map instead of yoinking from the DB
	err = unfuckRoutes(zipWriter, usageMap)
	if err != nil {
		return err
	}

	err = unfuckStopTimes(zipWriter, allTrips)
	if err != nil {
		return err
	}

	err = rewriteFeedInfo(f, zipWriter)
	if err != nil {
		return err
	}

	err = rewriteStops(f, zipWriter)
	if err != nil {
		return err
	}

	//TODO do attributions otherwise things are gonna get legally spicy
	//TODO unfuck calendar dates (doesn't *strictly* go against the spec but yeah, still urgh)
	//FIXME that's a temp solution, not guaranteed to work
	err = rewriteCalendarDates(f, zipWriter)
	if err != nil {
		return err
	}

	err = rewriteAgency(f, zipWriter)
	if err != nil {
		return err
	}

	return nil
	//TODO: trips, stops, stop times, routes, feed_info, agency, (calendar, calendar dates)
}

func rewriteAgency(f *trainmapdb.Fetcher, zipWriter *zip.Writer) error {
	agenciesWithDuplicates, err := f.GetAllAgencies()
	if err != nil {
		return err
	}
	agencyMap := make(map[string]trainmapdb.Agency)
	for _, agency := range agenciesWithDuplicates {
		agencyMap[agency.AgencyId] = agency
	}
	var agenciesWithoutDuplicates []trainmapdb.Agency
	for _, agency := range agencyMap {
		agenciesWithoutDuplicates = append(agenciesWithoutDuplicates, agency)
	}
	csvContent, err := csvutil.Marshal(agenciesWithoutDuplicates)
	if err != nil {
		return err
	}
	writer, err := zipWriter.Create("agency.txt")
	if err != nil {
		return err
	}
	_, err = writer.Write(csvContent)
	if err != nil {
		return err
	}
	return nil
}

func rewriteStops(f *trainmapdb.Fetcher, zipWriter *zip.Writer) error {
	stopsWithDuplicates, err := f.GetAllStops()
	if err != nil {
		return err
	}
	stopMap := make(map[string]trainmapdb.Stop)
	for _, stop := range stopsWithDuplicates {
		stopMap[stop.StopId] = stop
	}
	var stopsWithoutDuplicates []trainmapdb.Stop
	for _, stop := range stopMap {
		stopsWithoutDuplicates = append(stopsWithoutDuplicates, stop)
	}
	csvContent, err := csvutil.Marshal(stopsWithoutDuplicates)
	if err != nil {
		return err
	}
	writer, err := zipWriter.Create("stops.txt")
	if err != nil {
		return err
	}
	_, err = writer.Write(csvContent)
	if err != nil {
		return err
	}

	return nil
}

func rewriteCalendarDates(f *trainmapdb.Fetcher, zipWriter *zip.Writer) error {
	ONE_DAY := 24 * time.Hour
	today := time.Now().Truncate(ONE_DAY)
	yesterday := today.Add(-ONE_DAY)
	yearFromNow := today.Add(365 * ONE_DAY)
	serviceDays, err := f.GetServicesBetweenDates(yesterday, yearFromNow)
	if err != nil {
		return err
	}
	var serviceExceptions []trainmapdb.CalendarDate
	//FIXME take feedId into account
	for _, serviceDay := range serviceDays {
		sex := trainmapdb.CalendarDate{
			ServiceId:     serviceDay.ServiceId,
			CsvDate:       serviceDay.Date.Format("20060102"),
			ExceptionType: trainmapdb.ExceptionTypeServiceAdded,
		}
		serviceExceptions = append(serviceExceptions, sex)
	}

	csvContent, err := csvutil.Marshal(serviceExceptions)
	if err != nil {
		return err
	}

	writer, err := zipWriter.Create("calendar_dates.txt")

	if err != nil {
		return err
	}

	_, err = writer.Write(csvContent)

	if err != nil {
		return err
	}

	return nil
}

func rewriteFeedInfo(f *trainmapdb.Fetcher, zipWriter *zip.Writer) error {
	feeds, err := f.GetFeeds()
	if err != nil {
		return err
	}
	onlyFirstFeed := feeds[:1]
	csvContent, err := csvutil.Marshal(onlyFirstFeed)
	if err != nil {
		return err
	}
	writer, err := zipWriter.Create("feed_info.txt")
	if err != nil {
		return err
	}
	_, err = writer.Write(csvContent)
	if err != nil {
		return err
	}
	return nil
}

func unfuckStopTimes(zipWriter *zip.Writer, allTrips []trainmapdb.Trip) error {
	//adjust the trip IDs to match the new IDs
	var unfuckedStopTimes []trainmapdb.StopTime
	for _, trip := range allTrips {
		for _, stopTime := range trip.StopTimes {
			stopTime.TripId = fmt.Sprintf("%s-%s", trip.FeedId, stopTime.TripId)
			stopTime.CsvDepartureTime = stopTime.DepartureTime.Format("15:04:05")
			stopTime.CsvArrivalTime = stopTime.ArrivalTime.Format("15:04:05")
			unfuckedStopTimes = append(unfuckedStopTimes, stopTime)
		}
	}

	csvContent, err := csvutil.Marshal(unfuckedStopTimes)
	if err != nil {
		return err
	}

	fileWriter, err := zipWriter.Create("stop_times.txt")
	if err != nil {
		return err
	}

	_, err = fileWriter.Write(csvContent)
	if err != nil {
		return err
	}

	return nil
}

func unfuckRoutes(zipWriter *zip.Writer, usageMap map[feededRouteId]routeUsageStatus) error {
	var unfuckedRoutes []trainmapdb.Route
	//duplicate by status
	for _, status := range usageMap {
		if status.hasTram {
			copy := status.route
			copy.RouteId = fmt.Sprintf("%s-%s-%s", copy.FeedId, routeTypePrefixMapping[trainmapdb.RouteTypeTram], copy.RouteId)
			copy.RouteType = trainmapdb.RouteTypeTram
			unfuckedRoutes = append(unfuckedRoutes, copy)
		}
		if status.hasHeavyRail {
			copy := status.route
			copy.RouteId = fmt.Sprintf("%s-%s-%s", copy.FeedId, routeTypePrefixMapping[trainmapdb.RouteTypeHeavyRail], copy.RouteId)
			copy.RouteType = trainmapdb.RouteTypeHeavyRail
			unfuckedRoutes = append(unfuckedRoutes, copy)
		}
		if status.hasBus {
			copy := status.route
			copy.RouteId = fmt.Sprintf("%s-%s-%s", copy.FeedId, routeTypePrefixMapping[trainmapdb.RouteTypeBus], copy.RouteId)
			copy.RouteType = trainmapdb.RouteTypeBus
			unfuckedRoutes = append(unfuckedRoutes, copy)
		}
	}

	csvContent, err := csvutil.Marshal(unfuckedRoutes)
	if err != nil {
		return err
	}
	routeWriter, err := zipWriter.Create("routes.txt")
	if err != nil {
		return err
	}
	_, err = routeWriter.Write(csvContent)
	if err != nil {
		return err
	}
	return nil
}

func unfuckTrips(f *trainmapdb.Fetcher, zipWriter *zip.Writer, usageMap *map[feededRouteId]routeUsageStatus) ([]trainmapdb.Trip, error) {
	allTrips, err := f.GetAllTrips()
	if err != nil {
		return nil, err
	}
	var unfuckedTrips []trainmapdb.Trip
	oldTripMap := make(map[feededTripId]trainmapdb.Trip, len(allTrips))
	for _, trip := range allTrips {
		unfuckedTrips = append(unfuckedTrips, unfuckTrip(trip, usageMap))
		ftid := feededTripId{feedId: trip.FeedId, tripId: trip.TripId}
		oldTripMap[ftid] = trip
	}
	csvContent, err := csvutil.Marshal(unfuckedTrips)
	if err != nil {
		return nil, err
	}
	tripsFile, err := zipWriter.Create("trips.txt")
	if err != nil {
		return nil, err
	}
	_, err = tripsFile.Write(csvContent)
	if err != nil {
		return nil, err
	}

	return allTrips, nil
}

type feededTripId struct {
	feedId, tripId string
}

func unfuckTrip(trip trainmapdb.Trip, usageMap *map[feededRouteId]routeUsageStatus) trainmapdb.Trip {
	routeType := getTripRouteType(trip)

	//store data on usage map
	frid := feededRouteId{feedId: trip.FeedId, routeId: trip.RouteId}
	//get status
	status := (*usageMap)[frid]
	status.route = *trip.Route //store route
	switch routeType {
	case trainmapdb.RouteTypeBus:
		status.hasBus = true
	case trainmapdb.RouteTypeTram:
		status.hasTram = true
	case trainmapdb.RouteTypeHeavyRail:
		status.hasHeavyRail = true
	}
	//store status when done
	(*usageMap)[frid] = status

	//now actually unfuck the headsign UwU
	trip.TripShortName = trip.Headsign
	trip.Headsign = trip.StopTimes[len(trip.StopTimes)-1].Stop.StopName
	trip.RouteId = fmt.Sprintf("%s-%s-%s", trip.FeedId, routeTypePrefixMapping[routeType], trip.RouteId)
	trip.TripId = fmt.Sprintf("%s-%s", trip.FeedId, trip.TripId)

	return trip
}

func getStopRouteType(stop trainmapdb.Stop) trainmapdb.RouteType {
	//check for SNCF's prefixes in the stop ID to determine the route
	//yes, i know, this is horrible

	busPrefixes := []string{"Car"}
	tramPrefixes := []string{"TramTrain"}

	lowerId := strings.ToLower(stop.StopId)

	for _, busPrefix := range busPrefixes {
		if strings.Contains(lowerId, strings.ToLower(busPrefix)) {
			return trainmapdb.RouteTypeBus
		}
	}
	for _, tramTrainPrefix := range tramPrefixes {
		if strings.Contains(lowerId, strings.ToLower(tramTrainPrefix)) {
			return trainmapdb.RouteTypeTram
		}
	}

	//be lazy, assume the rest has to be trains
	return trainmapdb.RouteTypeHeavyRail
}

// gets the trip's route type by inferring it from its stops' route types
func getTripRouteType(trip trainmapdb.Trip) trainmapdb.RouteType {
	if len(trip.StopTimes) == 0 {
		panic(fmt.Errorf("no stoptimes in the trip, this is bad and should never happen"))
	}

	stop := trip.StopTimes[0].Stop
	if stop == nil {
		panic(fmt.Errorf("nil stop, should never happen if the queries are done correctly"))
	}

	return getStopRouteType(*stop)
}
