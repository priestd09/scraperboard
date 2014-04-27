package scraperboard

import (
	"encoding/xml"
	"errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/golang/glog"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

func NewScraperFromString(str string) (Scraper, error) {
	return NewScraper(strings.NewReader(str))
}

func NewScraperFromFile(name string) (Scraper, error) {
	file, err := os.Open(name)
	if err != nil {
		return Scraper{}, err
	}
	return NewScraper(file)
}

func NewScraper(r io.Reader) (scraper Scraper, err error) {
	err = xml.NewDecoder(r).Decode(&scraper)
	return
}

func (s *Scraper) ScrapeFromUrl(url string) (result map[string]interface{}, err error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return
	}
	return s.scrape(doc)
}

func (s *Scraper) ScrapeFromResponse(res *http.Response) (result map[string]interface{}, err error) {
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return
	}
	return s.scrape(doc)
}

func (s *Scraper) ScrapeFromReader(reader io.Reader) (result map[string]interface{}, err error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return
	}
	return s.scrape(doc)
}

func (s *Scraper) scrape(doc *goquery.Document) (result map[string]interface{}, err error) {
	var sel *goquery.Selection

	if s.Selector != "" {
		sel = doc.Filter(s.Selector)
	} else {
		sel = doc.Selection
	}

	result = make(map[string]interface{})
	var k string
	var v interface{}

	for _, each := range s.EachList {
		k, v, err = each.scrape(sel)
		if err != nil {
			return
		}
		result[k] = v
	}

	for _, property := range s.PropertyList {
		k, v, err = property.scrape(sel)
		if err != nil {
			return
		}
		result[k] = v
	}

	if s.Name != "" {
		result = map[string]interface{}{s.Name: result}
	}
	return
}

func (s *Each) scrape(sel *goquery.Selection) (key string, value []map[string]interface{}, err error) {
	find := sel.Find(s.Selector)
	key = s.Name
	value = make([]map[string]interface{}, find.Size())

	find.Each(func(i int, sel *goquery.Selection) {
		glog.Infof("Processing %s/%d", s.Name, i)
		if glog.V(3) {
			html, _ := sel.Html()
			glog.Infoln(html)
		}
		value[i] = make(map[string]interface{})
		for _, property := range s.PropertyList {
			k, v, err := property.scrape(sel)
			if err != nil {
				return
			}
			value[i][k] = v
		}
	})
	return
}

func (s *Property) scrape(sel *goquery.Selection) (key string, value interface{}, err error) {
	key = s.Name
	value = sel.Find(s.Selector)
	glog.Infof("Property %v from %v", s.Name, value)

	if sel.Find(s.Selector).Length() == 0 {
		glog.Info("No matches for ", s.Selector)
		value = nil
		return
	}

	if len(s.FilterList) == 0 {
		s.FilterList = defaultFilterList()
	}

	defer func() {
		if r := recover(); r != nil {
			glog.Error(r)
			glog.V(2).Infof("%s\n", debug.Stack())
		}
	}()

	glog.Infof("Passing filters on %v", s.Name)
	for _, filter := range s.FilterList {
		value, err = filter.run(value)
		if err != nil {
			return
		}
	}
	glog.Infof("Property %v: %v", s.Name, value)
	return
}

func (f *Filter) run(val interface{}) (result interface{}, err error) {
	switch f.Type {
	case "first":
		result = val.(*goquery.Selection).First()
	case "text":
		result = val.(*goquery.Selection).Text()
	case "attr":
		result, _ = val.(*goquery.Selection).Attr(f.Argument)
	case "regex":
		exp := regexp.MustCompile(f.Argument)
		result = exp.FindAllStringSubmatch(val.(string), 1)[0][1]
	case "parseDate":
		result, err = time.Parse(f.Argument, val.(string))
	default:
		err = errors.New("Unknown filter " + f.Type)
	}
	glog.Infof("FILTER %s (%s): %v", f.Type, f.Argument, result)
	return
}

func defaultFilterList() []Filter {
	return []Filter{
		Filter{Type: "first"},
		Filter{Type: "text"},
	}
}

type Scraper struct {
	Property
	EachList     []Each     `xml:"Each"`
	PropertyList []Property `xml:"Property"`
}

type Each struct {
	Property
	sortBy       string     `xml:"sortBy,attr"`
	PropertyList []Property `xml:"Property"`
}

type Property struct {
	Name       string   `xml:"name,attr"`
	Selector   string   `xml:"selector,attr"`
	FilterList []Filter `xml:"Filter"`
}

type Filter struct {
	Type     string `xml:"type,attr"`
	Argument string `xml:"argument,attr"`
}