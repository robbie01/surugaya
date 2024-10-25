package compare

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"

	_ "github.com/mattn/go-sqlite3"
)

type Change struct {
	Id           string
	OldName      string
	NewName      string
	OldCondition string
	NewCondition string
	OldPriceLo   uint
	NewPriceLo   uint
	OldPriceHi   uint
	NewPriceHi   uint
}

func Compare(oldDb string, newDb string) (changes []Change) {
	fmt.Println((&url.URL{
		Scheme:   "file",
		Path:     newDb,
		RawQuery: "mode=ro",
	}).String())

	db, err := sql.Open("sqlite3", "file:"+(&url.URL{
		Path:     newDb,
		RawQuery: "mode=ro",
	}).String())
	if err != nil {
		log.Println("error opening new database:", err)
		return
	}
	defer db.Close()

	_, err = db.Exec("ATTACH DATABASE ? AS old", "file:"+(&url.URL{
		Path:     oldDb,
		RawQuery: "mode=ro",
	}).String())
	if err != nil {
		log.Println("error opening old database:", err)
		return
	}

	rs, err := db.Query(`SELECT id,
	    COALESCE(o.name, ''), COALESCE(n.name, ''),
		COALESCE(o.condition, ''), COALESCE(n.condition, ''),
		COALESCE(o.priceLo, 0), COALESCE(n.priceLo, 0),
		COALESCE(o.priceHi, 0), COALESCE(n.priceHi, 0)
		FROM main.products AS n FULL OUTER JOIN old.products AS o USING (id)
		WHERE o.id IS NULL or n.id IS NULL
		OR o.name <> n.name
		OR o.condition <> n.condition
		OR o.priceLo <> n.priceLo
		OR o.priceHi <> n.priceHi`)
	if err != nil {
		log.Println("error submitting query:", err)
		return
	}
	defer rs.Close()

	for rs.Next() {
		var change Change
		err := rs.Scan(
			&change.Id,
			&change.OldName,
			&change.NewName,
			&change.OldCondition,
			&change.NewCondition,
			&change.OldPriceLo,
			&change.NewPriceLo,
			&change.OldPriceHi,
			&change.NewPriceHi,
		)
		if err != nil {
			log.Println("error scanning row:", err)
			return
		}

		changes = append(changes, change)
	}

	return
}
