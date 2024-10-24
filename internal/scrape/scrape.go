package scrape

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	_ "github.com/mattn/go-sqlite3"
)

type product struct {
	id        string
	name      string
	condition string
	priceLo   uint64
	priceHi   uint64
}

func Scrape(pageTemplates []string, dbName string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	products := make(chan product)

	c := colly.NewCollector(
		colly.Async(true),
		//colly.CacheDir("cache"),
		colly.StdlibContext(ctx),
	)

	pageRe, err := regexp.Compile("page=([0-9]+)")
	if err != nil {
		log.Println("error compiling regex:", err)
		return
	}

	// Both of these URLs report 1822 entries, yet for some reason
	// they yield overlapping but distinct sets, and their union is still
	// only 1760 entries.

	c.OnHTML("a.page-link[href]", func(e *colly.HTMLElement) {
		// Predictively synthesize remaining links (major optimization)
		// Deduplication is not a concern here; colly handles this for us

		url := e.Attr("href")
		page, _ := strconv.ParseUint(pageRe.FindStringSubmatch(url)[1], 10, 64)
		for p := uint64(1); p <= page; p++ {
			for _, t := range pageTemplates {
				e.Request.Visit(fmt.Sprintf("%s%d", t, p))
			}
		}
	})

	c.OnHTML(".product_wrap", func(e *colly.HTMLElement) {
		id := e.ChildAttr("[data-product-id]", "data-product-id")
		name := strings.TrimSpace(e.ChildText(".title_product"))

		var condition string
		var priceLo string
		var priceHi string
		if strings.Contains(e.ChildText(".price_product"), "品切れ") {
			// out of stock
			condition = "品切れ"
		} else {
			conditions := strings.Fields(e.ChildText(".message"))
			slices.Sort(conditions)
			condition = strings.Join(conditions, " ")

			priceLo = e.ChildAttr("[data-price]", "data-price")
			if priceLo == "" {
				priceLo = e.ChildAttr("[data-price-from]", "data-price-from")
				priceHi = e.ChildAttr("[data-price-to]", "data-price-to")
			}
		}

		ipriceLo, err := strconv.ParseUint(priceLo, 10, 64)
		if err != nil && priceLo != "" {
			log.Println("error parsing priceLo:", err)
			cancel()
			return
		}

		ipriceHi, err := strconv.ParseUint(priceHi, 10, 64)
		if err != nil && priceHi != "" {
			log.Println("error parsing priceHi:", err)
			cancel()
			return
		}

		products <- product{
			id:        id,
			name:      name,
			condition: condition,
			priceLo:   ipriceLo,
			priceHi:   ipriceHi,
		}
	})

	c.OnRequest(func(r *colly.Request) {
		//log.Println("Visiting", r.URL)
	})

	os.Remove(dbName)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", dbName))
	if err != nil {
		log.Println("error opening database:", err)
		return
	}
	defer db.Close()

	context.AfterFunc(ctx, func() {
		// Drain products on cancel
		for range products {
		}
	})

	go func() {
		defer cancel()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			log.Println("error beginning tx:", err)
			return
		}

		_, err = tx.ExecContext(ctx, "CREATE TABLE products(id TEXT PRIMARY KEY, name TEXT NOT NULL, condition TEXT NOT NULL, priceLo INTEGER NOT NULL, priceHi INTEGER NOT NULL) WITHOUT ROWID, STRICT")
		if err != nil {
			log.Println("error creating table:", err)
			return
		}

		stmt, err := tx.PrepareContext(ctx, "INSERT INTO products VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Println("error preparing statement:", err)
			return
		}

		uniq := make(map[string]struct{})
		for prod := range products {
			key := prod.id
			if _, ok := uniq[key]; !ok {
				uniq[key] = struct{}{}

				_, err = stmt.ExecContext(ctx, prod.id, prod.name, prod.condition, prod.priceLo, prod.priceHi)
				if err != nil {
					log.Println("error inserting product:", err)
					return
				}
			}
		}

		if ctx.Err() == nil {
			err = tx.Commit()
			if err != nil {
				log.Println("error committing tx:", err)
				return
			}

			log.Println("successfully wrote products database")
		}
	}()

	log.Println("Visiting begin")
	for _, t := range pageTemplates {
		c.Visit(fmt.Sprintf("%s1", t))
	}

	c.Wait()
	close(products)
	log.Println("Visiting finished")

	<-ctx.Done()

	err = db.Close()
	if err != nil {
		log.Println("error closing database:", err)
	}
}
