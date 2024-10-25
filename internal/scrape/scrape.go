package scrape

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/gocolly/colly/v2"
	_ "github.com/mattn/go-sqlite3"
)

type product struct {
	id        string
	name      string
	condition string
	priceLo   uint
	priceHi   uint
}

const intBits int = int(unsafe.Sizeof(1) * 8)

func Scrape(pageTemplates []string, dbName string) {
	ctx, cancel := context.WithCancelCause(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		cancel(fmt.Errorf("interrupted"))
	}()

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

	c.OnHTML("a.page-link[href]", func(e *colly.HTMLElement) {
		// Predictively synthesize remaining links (major optimization)
		// Deduplication is not a concern here; colly handles this for us

		url := e.Attr("href")
		page, err := strconv.ParseUint(pageRe.FindStringSubmatch(url)[1], 10, intBits)
		if err != nil {
			cancel(fmt.Errorf("failed to parse page link at %s: %w", url, err))
		}

		for p := uint(1); p <= uint(page); p++ {
			e.Request.Visit(fmt.Sprintf("%s%d", e.Request.Ctx.Get("template"), p))
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

		if id == "" {
			cancel(fmt.Errorf("id is empty at %s", e.Request.URL.String()))
			return
		}

		ipriceLo, err := strconv.ParseUint(priceLo, 10, intBits)
		if err != nil && priceLo != "" {
			cancel(fmt.Errorf("error parsing priceLo at %s: %w", e.Request.URL.String(), err))
			return
		}

		ipriceHi, err := strconv.ParseUint(priceHi, 10, intBits)
		if err != nil && priceHi != "" {
			cancel(fmt.Errorf("error parsing priceHi at %s: %w", e.Request.URL.String(), err))
			return
		}

		products <- product{
			id:        id,
			name:      name,
			condition: condition,
			priceLo:   uint(ipriceLo),
			priceHi:   uint(ipriceHi),
		}
	})

	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL)
	})

	os.Remove(dbName)
	db, err := sql.Open("sqlite3", "file:"+(&url.URL{
		Path:     dbName,
		RawQuery: "mode=rwc",
	}).String())
	if err != nil {
		log.Println("error opening database:", err)
		return
	}

	context.AfterFunc(ctx, func() {
		// Drain products on cancel to unblock colly
		for range products {
		}
	})

	go func() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			cancel(fmt.Errorf("error beginning tx: %w", err))
			return
		}

		_, err = tx.ExecContext(ctx, `CREATE TABLE products(
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			condition TEXT NOT NULL,
			priceLo INTEGER NOT NULL,
			priceHi INTEGER NOT NULL
		) WITHOUT ROWID, STRICT`)
		if err != nil {
			cancel(fmt.Errorf("error creating table: %w", err))
			return
		}

		stmt, err := tx.PrepareContext(ctx, "INSERT INTO products VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			cancel(fmt.Errorf("error preparing statement: %w", err))
			return
		}

		uniq := make(map[string]struct{})
		for prod := range products {
			key := prod.id
			if _, ok := uniq[key]; !ok {
				uniq[key] = struct{}{}

				_, err = stmt.ExecContext(ctx, prod.id, prod.name, prod.condition, prod.priceLo, prod.priceHi)
				if err != nil {
					cancel(fmt.Errorf("error inserting product: %w", err))
					return
				}
			}
		}

		if ctx.Err() == nil {
			err = tx.Commit()
			if err != nil {
				cancel(fmt.Errorf("error committing tx: %w", err))
				return
			}

			cancel(nil) // The only clean exit point
		}
	}()

	log.Println("Visiting begin")
	for _, t := range pageTemplates {
		reqCtx := colly.NewContext()
		reqCtx.Put("template", t)

		c.Request(
			"GET",
			fmt.Sprintf("%s1", t),
			nil,
			reqCtx,
			nil,
		)
	}

	c.Wait()
	close(products)

	<-ctx.Done()

	err = context.Cause(ctx)
	if err == context.Canceled {
		err = db.Close()
		if err != nil {
			log.Println("error closing database:", err)
			return
		}

		log.Println("successfully wrote products database")
	} else {
		log.Println(err)
	}
}
