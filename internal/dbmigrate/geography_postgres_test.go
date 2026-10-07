package dbmigrate_test

import (
	"barber-booking/internal/dbmigrate"
	"barber-booking/internal/geography"
	"context"
	"os"
	"testing"
)

func TestPostgresCanonicalGeography(t *testing.T) {
	conn := migrationDatabase(t)
	schema, err := os.ReadFile("../../supabase/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`insert into public.barbers(full_name,phone_number,email,password_hash,shop_name,shop_address,state,local_area)
 values ('Legacy','123','legacy@example.com','hash','Shop','Address','oGuN state','Egbado North'),
 ('Unknown','123','unknown@example.com','hash','Shop','Address','LAGOS','Victoria Island'),
 ('Invalid state','123','unresolved@example.com','hash','Shop','Address','Old state','Old area')`); err != nil {
		t.Fatal(err)
	}
	if err = dbmigrate.Apply(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	var state, area string
	var stateID, areaID *string
	if err = conn.QueryRow(`select state,local_area,state_id,local_area_id from public.barbers where email='legacy@example.com'`).Scan(&state, &area, &stateID, &areaID); err != nil || state != "Ogun" || area != "Yewa North" || stateID == nil || areaID == nil {
		t.Fatalf("backfill %s/%s %v %v %v", state, area, stateID, areaID, err)
	}
	if err = conn.QueryRow(`select state,local_area,state_id,local_area_id from public.barbers where email='unknown@example.com'`).Scan(&state, &area, &stateID, &areaID); err != nil || state != "Lagos" || area != "Victoria Island" || stateID == nil || areaID != nil {
		t.Fatalf("guessed legacy area %s/%s %v %v %v", state, area, stateID, areaID, err)
	}
	if err = conn.QueryRow(`select state,local_area,state_id,local_area_id from public.barbers where email='unresolved@example.com'`).Scan(&state, &area, &stateID, &areaID); err != nil || state != "Old state" || area != "Old area" || stateID != nil || areaID != nil {
		t.Fatalf("lost unresolved text %s/%s %v", state, area, err)
	}
	var count int
	if err = conn.QueryRow(`select count(*) from degosalon_internal.geography_areas`).Scan(&count); err != nil || count != 774 {
		t.Fatalf("area count=%d %v", count, err)
	}
	if err = conn.QueryRow(`select count(*) from degosalon_internal.geography_states`).Scan(&count); err != nil || count != 37 {
		t.Fatalf("state count=%d %v", count, err)
	}
	for _, s := range geography.States() {
		var name string
		if err = conn.QueryRow(`select name from degosalon_internal.geography_states where id=$1`, s.ID).Scan(&name); err != nil || name != s.Name {
			t.Fatalf("state mismatch %s %v", s.ID, err)
		}
		for _, alias := range append([]string{s.ID, s.Name}, s.Aliases...) {
			var id string
			if err = conn.QueryRow(`select state_id from degosalon_internal.geography_state_aliases where alias=regexp_replace(lower($1),'[^a-z0-9]','','g')`, alias).Scan(&id); err != nil || id != s.ID {
				t.Fatalf("state alias mismatch %s %v", alias, err)
			}
		}
		for _, a := range s.Areas {
			if err = conn.QueryRow(`select name from degosalon_internal.geography_areas where state_id=$1 and id=$2`, s.ID, a.ID).Scan(&name); err != nil || name != a.Name {
				t.Fatalf("area mismatch %s %v", a.ID, err)
			}
			for _, alias := range append([]string{a.ID, a.Name}, a.Aliases...) {
				var id string
				if err = conn.QueryRow(`select area_id from degosalon_internal.geography_area_aliases where state_id=$1 and alias=regexp_replace(lower($2),'[^a-z0-9]','','g')`, s.ID, alias).Scan(&id); err != nil || id != a.ID {
					t.Fatalf("area alias mismatch %s/%s %v", s.ID, alias, err)
				}
			}

		}
	}
	if _, err = conn.Exec(`update public.barbers set state='NG015',local_area='Abuja Municipal' where email='legacy@example.com'`); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(`select state,local_area,state_id,local_area_id from public.barbers where email='legacy@example.com'`).Scan(&state, &area, &stateID, &areaID); err != nil || state != "FCT" || area != "Abuja Municipal" || areaID == nil {
		t.Fatalf("trigger %s/%s %v", state, area, err)
	}
	if _, err = conn.Exec(`insert into degosalon_internal.geography_areas(id,state_id,name) values ('invalid','absent','Invalid')`); err == nil {
		t.Fatal("missing parent allowed")
	}
	if _, err = conn.Exec(`insert into degosalon_internal.geography_area_aliases values('NG025','invalid','NG015001')`); err == nil {
		t.Fatal("cross-state alias allowed")
	}
}
