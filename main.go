package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// Парсим динамический сайт https://scrapingclub.com/exercise/list_infinite_scroll/
// Задача: параллельно собрать название, цену и описание товара внутри каждой карточки.
// Будем использовать chromedp для парсинга динамического сайта, используя скроллинг
// и goquery для сбора CSS-селекторов с HTML.

// Достаем с сайта html через скроллинг и ожидание всех товаров
func fetchHtmlWithScrolling(url string) (string, error) {

	fmt.Println("--INFO: Start fetchHtmlWithScrolling")

	// Скрипт скроллинга JS
	scriptForScrolling := `

	const numberScrolls = 10

	let countScroll = 0

	const interval = setInterval(() => {

	window.scrollTo(0, document.body.scrollHeight)

	countScroll++

	if (countScroll === numberScrolls) {

	clearInterval(interval)

	}

	}, 1000)

	`

	fmt.Println("--INFO: Open site")

	ctx, cancel := chromedp.NewContext(
		context.Background(),
	)

	defer cancel()

	var htmlContent string

	fmt.Println("--INFO: Get html page")

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Evaluate(scriptForScrolling, nil),
		chromedp.WaitVisible(".post:nth-child(60)"),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		log.Fatal("Error: ", err)
		return "", err
	}

	fmt.Println("--INFO: Finish parse html page")

	return htmlContent, nil
}

// Достаем с сайта html без скроллинга
func fetchHtml(url string) (string, error) {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
	)

	defer cancel()

	var htmlContent string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		log.Fatal("Error: ", err)
		return "", err
	}

	return htmlContent, nil
}

// Собираем все ссылки в слайс через html главной страницы сайта
func parseUrls(htmlContent string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))

	if err != nil {
		log.Fatal(err)
	}

	urls := []string{}

	doc.Find("div.post").Each(func(i int, s *goquery.Selection) {
		urls = append(urls, fmt.Sprintf("https://scrapingclub.com%v", s.Find("a").AttrOr("href", "non-href")))
	})

	return urls
}

// Собираем имя, цену и описание товара через html
func parseProduct(html string) (string, string, string) {

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Fatal(err)
		return "", "", ""
	}

	name := doc.Find("h3.card-title").Text()
	price := doc.Find("h4.card-price").Text()
	desc := doc.Find("p.card-description").Text()

	return name, price, desc
}

func main() {
	// Создаем файл
	fname := "products"
	file, err := os.Create(fmt.Sprintf("./%v.csv", fname))
	if err != nil {
		log.Fatalf("Cannot create file %v: %s\n", file, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	defer writer.Flush()
	// Записываем headers
	writer.Write([]string{"Name", "Price", "Description"})

	// Начинаем с сбора html главного сайта через скроллинг
	htmlContentWithScroll, err := fetchHtmlWithScrolling("https://scrapingclub.com/exercise/list_infinite_scroll/")
	if err != nil {
		log.Fatal("Error get html: ", err)
	}

	// Собираем все ссылки на товары по полученному html
	urls := parseUrls(htmlContentWithScroll)

	htmlContents := []string{}

	var wg sync.WaitGroup

	// Параллельно собираем все html товаров в горутинах и записываем в слайс
	for _, url := range urls {
		fmt.Printf("--INFO: Start scraping %v\n", url)

		wg.Add(1)
		go func(page string) {
			defer wg.Done()
			html, err := fetchHtml(page)
			if err != nil {
				log.Fatal("Error: ", err)
			}
			htmlContents = append(htmlContents, html)
			fmt.Printf("--INFO: Finish parse %v\n", page)
			fmt.Println("----------------------------------")
		}(url)
	}

	wg.Wait()

	// Собираем данные с html о всех продуктах и записываем в CSV файл
	for _, html := range htmlContents {
		fmt.Println("--INFO: Start parse html")

		name, price, desc := parseProduct(html)

		writer.Write([]string{
			name,
			price,
			desc,
		})

		fmt.Println("--INFO: Finish parse html")
		fmt.Println("----------------------------------")
	}

	log.Println("--INFO: Scraping finished, check the files!")
}
