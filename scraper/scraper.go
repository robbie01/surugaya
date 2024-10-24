package main

import (
	"os"

	_ "github.com/mattn/go-sqlite3"
	"sohio.net/surugaya/internal/scrape"
)

func main() {
	pageTemplates := []string{
		//"http://suruga-ya.com/ja/category/5010900?page=",               // all dolls
		//"https://www.suruga-ya.com/ja/products?category=5010900&page=", // all dolls 2
		"https://www.suruga-ya.com/ja/products?category=50109&keyword=%E3%82%B5%E3%82%AF%E3%83%A9%E5%A4%A7%E6%88%A6&page=",                            // Sakura Wars dolls
		"https://www.suruga-ya.com/ja/products?keyword=%E3%83%97%E3%83%AA%E3%83%91%E3%82%B9&page=",                                                    // PriPass
		"https://www.suruga-ya.com/ja/products?category=50109&keyword=%E5%A4%9C%E6%98%8E%E3%81%91%E5%89%8D%E7%91%A0%E7%92%83%E8%89%B2%E3%81%AA&page=", // Brighter than the dawning blue dolls
	}

	scrape.Scrape(pageTemplates, os.Args[1])
}
