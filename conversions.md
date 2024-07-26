# Conversions done by the Unfucker™
This lists the conversions done by this program; all fields that aren't mentioned remain unchanged. (changed IDs also get changed in foreign keys of course)

## Trips
| Field             | Old Value    | New Value                                         |
| ----------------- | ------------ | ------------------------------------------------- |
| `trip_headsign`   | Train number | Name of the last station (taken from `stops.txt`) |
| `trip_short_name` | *unused*     | Train number  (previously the trip headsign)      |
| `route_id`        | `<oldID>`    | `<feedId>-<r\|b>-<oldId>`                         |
| `trip_id`         | `<oldID>`    | `<feedID>-<oldID>`                                |

## Routes
$\rightarrow$ Routes get duplicated, once as buses and once as trains (if they're hybrid, that is)
| Field        | Old Value                      | New Value                                            |
| ------------ | ------------------------------ | ---------------------------------------------------- |
| `route_id`   | `<oldID>`                      | `<feedID>-<r\|b>-<oldID>`                                     |
| `route_type` | `2` (everything as heavy rail) | `2`/`3` (for the `r`-Variant vs for the `b`-Variant) |

## Services
> ***i*** TODO