package gmaps

import "testing"

func TestParseDirectionsPathAndData(t *testing.T) {
	// Two stops searched by name (their points only in the data blob), one pinned in the path, and
	// motorways and tolls avoided; the shape Google Maps web shares.
	link := "https://www.google.com/maps/dir/Granollers,+Barcelona/Sant+Celoni,+Barcelona/41.8009,2.3476/@41.7,2.3,11z/" +
		"data=!3m1!4b1!4m18!4m17!1m5!1m1!1s0x12a4c8b2:0x1234!2m2!1d2.2874!2d41.6079!1m5!1m1!1s0x12a4b1:0x5678!2m2!1d2.4925!2d41.6895!1m0!2m2!1b1!2b1!3e0"
	d, err := ParseDirections(link)
	if err != nil {
		t.Fatal(err)
	}
	want := []Waypoint{
		{Name: "Granollers, Barcelona", Lat: 41.6079, Lng: 2.2874, HasPoint: true},
		{Name: "Sant Celoni, Barcelona", Lat: 41.6895, Lng: 2.4925, HasPoint: true},
		{Lat: 41.8009, Lng: 2.3476, HasPoint: true},
	}
	if len(d.Waypoints) != len(want) {
		t.Fatalf("%+v", d.Waypoints)
	}
	for i := range want {
		if d.Waypoints[i] != want[i] {
			t.Errorf("stop %d: %+v, want %+v", i, d.Waypoints[i], want[i])
		}
	}
	if !d.Avoid.Highways || !d.Avoid.Tolls || d.Avoid.Ferries {
		t.Errorf("avoid %+v", d.Avoid)
	}
}

func TestParseDirectionsKeepsNamesWithoutPoints(t *testing.T) {
	d, err := ParseDirections("https://www.google.com/maps/dir/Mi+ubicaci%C3%B3n/Viladrau/41.80,2.34/data=!4m2!4m1!3e0")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Waypoints[0].Here || d.Waypoints[1].Name != "Viladrau" || d.Waypoints[1].HasPoint || !d.Waypoints[2].HasPoint {
		t.Errorf("%+v", d.Waypoints)
	}
}

func TestParseDirectionsAPIForm(t *testing.T) {
	d, err := ParseDirections("https://www.google.com/maps/dir/?api=1&origin=41.6,2.29&destination=Viladrau&waypoints=Sant+Celoni|41.7,2.4&avoid=tolls|ferries")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Waypoints) != 4 || d.Waypoints[1].Name != "Sant Celoni" || d.Waypoints[3].Name != "Viladrau" || !d.Avoid.Tolls || !d.Avoid.Ferries || d.Avoid.Highways {
		t.Errorf("%+v", d)
	}
}

func TestParseDirectionsRefusesOthers(t *testing.T) {
	for _, link := range []string{
		"https://www.google.com/maps/place/Viladrau",
		"https://example.com/maps/dir/41.6,2.29/41.7,2.4",
		"https://www.google.com/maps/dir/Viladrau/",
		"https://www.google.com/maps/dir/41.6,2.29/41.7,2.4/data=!4m9!4m8!1m5",
	} {
		d, err := ParseDirections(link)
		if link == "https://www.google.com/maps/dir/41.6,2.29/41.7,2.4/data=!4m9!4m8!1m5" {
			// A cut-off data blob must not break the stops the path already gives.
			if err != nil || len(d.Waypoints) != 2 {
				t.Errorf("truncated blob: %+v %v", d, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: want an error, got %+v", link, d)
		}
	}
}

// Read off Google Maps web on 2026-10-28 for Granollers, Sant Celoni and a pinned point.
func TestParseDirectionsRealLink(t *testing.T) {
	d, err := ParseDirections("https://www.google.com/maps/dir/Granollers,+Barcelona/San+Celoni,+Barcelona/41.8009,2.3476/@41.6970602,2.2531864,11z/" +
		"data=!4m14!4m13!1m5!1m1!1s0x12a4c7cf4242446d:0x1c7b5f146fbca228!2m2!1d2.2870889!2d41.6061927!1m5!1m1!1s0x12bb330254990d71:0x6ece69ad8948ff70!2m2!1d2.5495112!2d41.6826815!1m0?entry=ttu")
	if err != nil {
		t.Fatal(err)
	}
	want := []Waypoint{
		{Name: "Granollers, Barcelona", Lat: 41.6061927, Lng: 2.2870889, HasPoint: true},
		{Name: "San Celoni, Barcelona", Lat: 41.6826815, Lng: 2.5495112, HasPoint: true},
		{Lat: 41.8009, Lng: 2.3476, HasPoint: true},
	}
	for i := range want {
		if i >= len(d.Waypoints) || d.Waypoints[i] != want[i] {
			t.Fatalf("%+v", d.Waypoints)
		}
	}
}
