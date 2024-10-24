package main

import (
	_ "github.com/mattn/go-sqlite3"
	"sohio.net/surugaya/internal/scrape"
)

func main() {
	pageTemplates := []string{
		"https://www.suruga-ya.com/ja/products?category=50109&keyword=%E3%82%B5%E3%82%AF%E3%83%A9%E5%A4%A7%E6%88%A6&page=", // Sakura Wars dolls
		"https://www.suruga-ya.com/ja/products?keyword=%E3%83%97%E3%83%AA%E3%83%91%E3%82%B9&page=",                         // PriPass
	}

	scrape.Scrape(pageTemplates, "products.db")
}
