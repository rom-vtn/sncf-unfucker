# SNCF Unfucker ⚙

This project is born from my frustration from working with SNCF GTFS feeds, which do not follow the GTFS spec.

# Context 🚈
- In order to understand these ramblings you should probably get familiar with [the GTFS schedule spec](https://gtfs.org/schedule/reference/) first.
- SNCF allows the free download and use of their GTFS feeds under the ODBL License:
    - [TER feed (regional trains)](https://data.sncf.com/explore/dataset/sncf-ter-gtfs/information/)
    - [IC feed (intercity trains)](https://data.sncf.com/explore/dataset/sncf-intercites-gtfs/information/)
    - [TGV feed (high speed trains)](https://data.sncf.com/explore/dataset/horaires-des-train-voyages-tgvinouiouigo/information/)

> However, these feeds do *not* follow the spec (Google Maps users will probably have already noticed trains that go to numbers instead of cities).

- Therefore, my plan is to make a converter to fix these issues by turning the default SNCF feed into a ✨pretty feed✨

## What SNCF does wrong ❌ 
- `trip.trip_headsign` contains a train number and not a destination (they don't use `trip.trip_short_name` instead)
- `route_type` is defined at the route level instead of the trip level. This mostly means that hybrid bus/train routes are shown as train only
- The actual route type comes from the `stop.stop_id` prefixes (`OCETrain`, `OCECar`, `OCEInoui`, `OCEOuigo`, ...)
- No stable `calendar` even though most routes run on a known schedule, everything is stored as `calendar_dates` (not really extra hamful, but it's frustrating to have to deal with)

# Usage
Just build and run the following: 
```bash
./sncf-unfucker <config.json> <output.zip>
```

# TODO 📋
- [X] Split route types by actual route type
- [X] Set `trip.trip_headsign` to the actual destination, and put the train number in `trip.trip_short_name`
- [X] Actually have a valid GTFS output
- [ ] Give stable calendars whenever possible
- [ ] Add `attributions.txt` in the output file to avoid legally spicy stuff
- [X] Make sure calendar ID space is unique

# Attributions ⚖
- Credit goes to SNCF for their feeds, which are published under ODbL
