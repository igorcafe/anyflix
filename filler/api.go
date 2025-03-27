package filler

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/igorcafe/anyflix/errorsx"
)

type Episode struct {
	Number string
	Title  string
	Type   string
}

type Show struct {
	Name     string
	URL      string
	Episodes []Episode
}

func SearchShow(query string) (Show, error) {
	slog.Debug("filler.SearchShow", "query", query)

	res, err := http.Get("https://www.animefillerlist.com/shows")
	if err != nil {
		return Show{}, fmt.Errorf("failed to fetch shows: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return Show{}, fmt.Errorf("failed to fetch shows: status code %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return Show{}, fmt.Errorf("failed to parse HTML: %v", err)
	}

	var bestMatch Show
	var bestMatchScore int

	doc.Find("#ShowList li > a").Each(func(index int, item *goquery.Selection) {
		re := regexp.MustCompile(`\s*\(.*?\)`)
		name := re.ReplaceAllString(item.Text(), "")
		url, exists := item.Attr("href")
		if !exists {
			return
		}

		score := similarityScore(query, name)
		if score > bestMatchScore {
			bestMatchScore = score
			bestMatch = Show{
				Name: name,
				URL:  "https://www.animefillerlist.com" + url,
			}
		}
	})

	if bestMatchScore == 0 {
		return Show{}, errorsx.NotFound
	}

	showRes, err := http.Get(bestMatch.URL)
	if err != nil {
		return Show{}, fmt.Errorf("failed to fetch show page: %v", err)
	}
	defer showRes.Body.Close()

	if showRes.StatusCode != 200 {
		return Show{}, fmt.Errorf("failed to fetch show page: status code %d", showRes.StatusCode)
	}

	showDoc, err := goquery.NewDocumentFromReader(showRes.Body)
	if err != nil {
		return Show{}, fmt.Errorf("failed to parse show HTML: %v", err)
	}

	var episodes []Episode
	showDoc.Find(".EpisodeList tr").Each(func(i int, item *goquery.Selection) {
		if i == 0 {
			return
		}

		number := item.Find("td.Number").Text()
		title := item.Find("td.Title").Text()

		var kind string
		rawType := strings.TrimSpace(strings.ToLower(item.Find("td.Type").Text()))

		switch rawType {
		case "filler":
			kind = "filler"
		case "mixed canon/filler":
			kind = "mixed"
		default:
			kind = "canon"
		}

		episodes = append(episodes, Episode{
			Number: number,
			Title:  title,
			Type:   kind,
		})
	})

	bestMatch.Episodes = episodes
	return bestMatch, nil
}

func similarityScore(a, b string) int {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	a = re.ReplaceAllString(strings.ToLower(a), " ")
	b = re.ReplaceAllString(strings.ToLower(b), " ")

	aChunks := strings.Fields(a)
	bChunks := strings.Fields(b)

	if slices.Equal(aChunks, bChunks) {
		return 100
	}

	score := 0
	for _, aChunk := range aChunks {
		if slices.Contains(bChunks, aChunk) {
			score++
		} else {
			score--
		}
	}

	for _, bChunk := range bChunks {
		if !slices.Contains(aChunks, bChunk) {
			score--
		}
	}

	return score
}
