package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

var (
	wpfeedurl    = "http://town.plattekill.ny.us/category/minutes/feed/"
	outputFolder = ""
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Items   Items    `xml:"channel"`
}
type Items struct {
	XMLName  xml.Name `xml:"channel"`
	ItemList []Item   `xml:"item"`
}
type Item struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Attachment  Attachment `xml:"encoded"`
	Category    []string   `xml:"category"`
	PubDate     PubDate    `xml:"pubDate"`
}

type Attachment struct {
	URL      string
	Filename string
}

func (a *Attachment) String() string {
	return a.URL
}

var urlregex = regexp.MustCompile(`http(s)?://town.plattekill.ny.us/wp-content/uploads/(\d+)/(\d+)/(.*)+.pdf`)

func (a *Attachment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {

	var raw string
	if err := d.DecodeElement(&raw, &start); err != nil {
		log.Println("Could not decode xml element:", start)
		return err
	}

	// strip the attachment url from it
	results := urlregex.FindAllStringSubmatch(raw, -1)
	if results == nil {
		log.Println("Could not find match in:", raw)
		return nil
	} else if len(results) < 1 {
		log.Println("Empty matched set:", raw)
		return nil
	} else if len(results[0]) < 5 {
		log.Println("Expected at least 4 sub-matches:", raw)
		return nil
	}

	a.URL = results[0][0]
	a.Filename = results[0][4] + ".pdf"

	return nil
}

type PubDate struct {
	time *time.Time
}

func (date *PubDate) String() string {
	return fmt.Sprintf("%d%d%d-%d%d%d", date.time.Year(), date.time.Month(), date.time.Day(), date.time.Hour(), date.time.Minute(), date.time.Second())
}

func (date *PubDate) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw string
	if err := d.DecodeElement(&raw, &start); err != nil {
		log.Println("Could not decode xml element:", start)
		return nil
	}

	// parse it into our time format
	v, err := time.Parse(time.RFC1123Z, raw)
	if err != nil {
		log.Println("Could not format time:", raw)
		return nil
	}

	date.time = &v
	return nil
}

func main() {

	flag.StringVar(&wpfeedurl, "f", wpfeedurl, "Specifies the wordpress' atom feed url to parse.")
	flag.StringVar(&outputFolder, "o", outputFolder, "Specifies the output folder to save to.")
	flag.Parse()

	log.Println("INFO Requesting RSS Feed:", wpfeedurl)
	r, err := http.Get(wpfeedurl)
	if err != nil {
		log.Println("ERROR while HTTP GET to wpfeedurl:", err)
		return
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("ERROR while reading body:", err)
		return
	}

	log.Println("INFO Unmarshaling xml into structs")
	var data RSS
	if err := xml.Unmarshal(body, &data); err != nil {
		log.Println("ERROR while unmarshaling the rss:", err)
		return
	}

	log.Println("INFO Iterating over each entry to download the attachment")
	for _, item := range data.Items.ItemList {

		// build the filename we are going to save to
		var filename = item.PubDate.String()
		for _, cat := range item.Category {
			if cat != "Minutes" {
				filename = fmt.Sprintf("%s_%s.pdf", cat, filename)
				break
			}
		}
		//fmt.Printf("\t%s\n", filename)

		// do we already have the file?
		if _, err := os.Stat(filename); err == nil {
			log.Println("WARN File already exists, skipping:", filename)
			continue
		}

		// create the file to download to
		f, err := os.Create(outputFolder + filename)
		if err != nil {
			log.Printf("ERROR while creating file: %s Error: %s", filename, err)
			continue
		}
		defer f.Close()

		// download the attachment
		dr, err := http.Get(item.Attachment.URL)
		if err != nil {
			log.Println("ERROR while HTTP GET of url:", item.Attachment.URL)
			continue
		}
		defer dr.Body.Close()
		copied, err := io.Copy(f, dr.Body)
		if err != nil {
			log.Println("ERROR while io.Copy() to file:", err)
			continue
		}

		log.Printf("INFO Downloaded %s (%d)", outputFolder+filename, copied)
	}
}
