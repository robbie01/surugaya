package main

import (
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	_ "github.com/mattn/go-sqlite3"
)

type product struct {
	id        uint64
	name      string
	condition string
	priceLo   uint64
	priceHi   uint64
}

func main() {
	products := make(chan product)

	c := colly.NewCollector(colly.Async(true), colly.CacheDir("cache"))

	c.OnHTML("a.page-link[href]", func(e *colly.HTMLElement) {
		e.Request.Visit(e.Attr("href"))
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

		iid, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			log.Fatalln("error parsing id:", err)
		}

		ipriceLo, err := strconv.ParseUint(priceLo, 10, 64)
		if err != nil && priceLo != "" {
			log.Fatalln("error parsing priceLo:", err)
		}

		ipriceHi, err := strconv.ParseUint(priceHi, 10, 64)
		if err != nil && priceHi != "" {
			log.Fatalln("error parsing priceHi:", err)
		}

		products <- product{
			id:        iid,
			name:      name,
			condition: condition,
			priceLo:   ipriceLo,
			priceHi:   ipriceHi,
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	writingDone := make(chan struct{})

	go func() {
		db, err := sql.Open("sqlite3", "file:products.db")
		if err != nil {
			log.Fatalln("error opening database:", err)
		}

		_, err = db.Exec("CREATE TABLE products(id INTEGER PRIMARY KEY, name TEXT NOT NULL, condition TEXT NOT NULL, priceLo INTEGER NOT NULL, priceHi INTEGER NOT NULL) STRICT")
		if err != nil {
			log.Fatalln("error creating table:", err)
		}

		tx, err := db.Begin()
		if err != nil {
			log.Fatalln("error beginning tx:", err)
		}

		uniq := make(map[uint64]struct{})
		for prod := range products {
			key := prod.id
			if _, ok := uniq[key]; !ok {
				uniq[key] = struct{}{}

				_, err = tx.Exec("INSERT INTO products VALUES (?, ?, ?, ?, ?)", prod.id, prod.name, prod.condition, prod.priceLo, prod.priceHi)
				if err != nil {
					log.Fatalln("error inserting product:", err)
				}
			}
		}

		err = tx.Commit()
		if err != nil {
			log.Fatalln("error committing tx:", err)
		}

		err = db.Close()
		if err != nil {
			log.Fatalln("error closing database:", err)
		}
		close(writingDone)
	}()

	// Both of these URLs report 1822 entries, yet for some reason
	// they yield overlapping but distinct sets, and their union is still
	// only 1760 entries.
	c.Visit("https://www.suruga-ya.com/ja/category/5010900?page=1")
	c.Visit("https://www.suruga-ya.com/ja/products?category=5010900&page=1")

	c.Wait()
	close(products)
	<-writingDone
}
