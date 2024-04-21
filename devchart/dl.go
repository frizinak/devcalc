package devchart

import (
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const _burl = "https://www.digitaltruth.com"

var burl, _ = url.Parse(_burl)
var absURLRE = regexp.MustCompile(`^https?:`)
var safepathRE = regexp.MustCompile(`[^a-z0-9\._-]+`)
var noteRE = regexp.MustCompile(`(?i)note.*\[.*]$`)

func init() {
	gob.Register(Entry{})
	gob.Register(Options{})
}

type Entry struct {
	Name      string
	Developer string
	Dilution  string
	ISO       string
	T135      time.Duration
	T120      time.Duration
	TSheet    time.Duration
	Temp      float64
	Notes     []string
}

type Options struct {
	Developers []string
	Stocks     []string
}

type NotExistsError struct{ name string }

func (n NotExistsError) Error() string {
	return fmt.Sprintf("no such developer: '%s'", n.name)
}

func GetOptions(cacheDir string) (Options, error) {
	var o Options
	os.MkdirAll(cacheDir, 0755)
	cache := filepath.Join(cacheDir, "options")

	f, err := os.Open(cache)
	if err != nil {
		if os.IsNotExist(err) {
			o.Developers, o.Stocks, err = getOptions()
			if err != nil {
				return o, err
			}
			tmp := cache + ".tmp"
			f, err = os.Create(tmp)
			if err != nil {
				return o, err
			}

			enc := gob.NewEncoder(f)
			if err = enc.Encode(o); err != nil {
				f.Close()
				os.Remove(tmp)
				return o, err
			}
			f.Close()

			return o, os.Rename(tmp, cache)
		}
		return o, err
	}

	defer f.Close()
	dec := gob.NewDecoder(f)
	return o, dec.Decode(&o)
}

func Get(cacheDir string, dev string) ([]Entry, error) {
	os.MkdirAll(cacheDir, 0755)
	filename := strings.Trim(safepathRE.ReplaceAllString(strings.ToLower(dev), "-"), "-")
	sum := sha1.Sum([]byte(dev))
	filename = fmt.Sprintf("dev-%s-%s", filename, hex.EncodeToString(sum[:4]))
	cache := filepath.Join(cacheDir, filename)
	f, err := os.Open(cache)
	if err != nil {
		if os.IsNotExist(err) {
			es, err := get(dev)
			if err != nil {
				return es, err
			}
			tmp := cache + ".tmp"
			f, err = os.Create(tmp)
			if err != nil {
				return nil, err
			}

			enc := gob.NewEncoder(f)
			if err := enc.Encode(es); err != nil {
				f.Close()
				os.Remove(tmp)
				return nil, err
			}
			f.Close()

			return es, os.Rename(tmp, cache)
		}
		return nil, err
	}

	defer f.Close()
	dec := gob.NewDecoder(f)
	list := make([]Entry, 0)
	return list, dec.Decode(&list)
}

func getOptions() ([]string, []string, error) {
	u := *burl
	u.Path = "/devchart.php"
	res, err := http.Get(u.String())
	if err != nil {
		return nil, nil, err
	}
	r := res.Body

	defer r.Close()

	var data [2][]string
	for i := range data {
		data[i] = make([]string, 0, 50)
	}

	mode := 0
	z := html.NewTokenizer(r)
outer:
	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			break outer
		case tt == html.StartTagToken:
			t := z.Token()
			if t.Data == "option" && mode != 0 {
				for _, attr := range t.Attr {
					if attr.Key == "value" && attr.Val != "" && attr.Val != "searchbox" {
						data[mode-1] = append(data[mode-1], attr.Val)
						break
					}
				}
			}

			if t.Data != "select" {
				continue
			}

			for _, attr := range t.Attr {
				if attr.Key == "name" || attr.Key == "id" {
					switch strings.ToLower(attr.Val) {
					case "film":
						mode = 1
					case "developer":
						mode = 2
					}
				}
			}

		case tt == html.EndTagToken:
			t := z.Token()
			if t.Data == "select" {
				mode = 0
			}
		}
	}

	return data[1], data[0], nil
}

func get(dev string) ([]Entry, error) {
	q := url.Values{
		"Film":      []string{""},
		"Developer": []string{dev},
		"mdc":       []string{"Search"},
		"TempUnits": []string{"C"},
		"TimeUnits": []string{"D"},
	}
	u := *burl
	u.Path = "/devchart.php"
	u.RawQuery = q.Encode()
	res, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	r := res.Body

	defer r.Close()

	data := make([]*[]string, 0)
	var str string
	var row *[]string
	state := 0
	z := html.NewTokenizer(r)
outer:
	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			break outer
		case tt == html.StartTagToken:
			t := z.Token()
			switch {
			case t.Data == "table":
				state = 1
			case t.Data == "tr":
				l := 0
				if row != nil {
					l = len(*row)
				}
				_row := make([]string, 0, l)
				row = &_row
				data = append(data, row)

				state = 2
			case t.Data == "td":
				state = 3
				*row = append(*row, "")
			case t.Data == "a" && state == 3:
				for _, a := range t.Attr {
					if a.Key == "href" {
						u, err := href(a.Val)
						if err == nil {
							(*row)[len(*row)-1] = u.String()
						}
					}
				}
			}
		case tt == html.EndTagToken:
			t := z.Token()
			switch {
			case t.Data == "table":
				state = 0
			case t.Data == "tr":
				state = 1
			case t.Data == "td":
				state = 2
				(*row)[len(*row)-1] = str
				str = ""
			}
		case tt == html.TextToken:
			if state == 3 {
				t := z.Token()
				str = t.Data
			}
		}
	}

	dur := func(str string) time.Duration {
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return 0
		}

		return time.Duration(float64(time.Minute) * f)

	}

	entries := make([]Entry, 0, len(data))
	for _, row := range data {
		r := *row
		if len(r) != 9 {
			continue
		}

		dil := r[2]
		if dil == "stock" {
			dil = "1+0"
		}

		if strings.HasPrefix(dev, "HC-110") || strings.HasPrefix(dev, "Ilfotec") {
			switch dil {
			case "A":
				dil = "1+15"
			case "B":
				dil = "1+31"
			case "C":
				dil = "1+19"
			case "D":
				dil = "1+39"
			case "E":
				dil = "1+47"
			case "F":
				dil = "1+79"
			case "G":
				dil = "1+119"
			case "H":
				dil = "1+63"
			case "J":
				dil = "1+150"
			}
		}

		var temp float64 = 0
		t := r[7]
		if len(t) > 1 {
			l := t[len(t)-1]
			if l == 'c' || l == 'C' {
				t = t[:len(t)-1]
			}
			temp, err = strconv.ParseFloat(t, 64)
			if err != nil {
				temp = 0
			}
		}

		var notes []string
		if r[8] != "" {
			notes, _ = getNotes(r[8])
		}

		entry := Entry{
			Name:      r[0],
			Developer: r[1],
			Dilution:  dil,
			ISO:       r[3],
			T135:      dur(r[4]),
			T120:      dur(r[5]),
			TSheet:    dur(r[6]),
			Temp:      temp,
			Notes:     notes,
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, NotExistsError{name: dev}
	}

	return entries, nil
}

func getNotes(u string) ([]string, error) {
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	r := res.Body

	defer r.Close()

	data := make([]string, 0)
	state := 0
	z := html.NewTokenizer(r)
outer:
	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			break outer
		case tt == html.StartTagToken:
			t := z.Token()
			switch {
			case t.Data == "table":
				state = 0
				for _, a := range t.Attr {
					if a.Key == "class" && a.Val == "notenote" {
						state = 1
						break
					}
				}
			case t.Data == "tr" && state == 1:
				state = 2
			}
		case tt == html.EndTagToken:
			t := z.Token()
			switch {
			case t.Data == "table":
				state = 0
			case t.Data == "tr" && state == 2:
				state = 1
			}
		case tt == html.TextToken:
			if state == 2 {
				t := z.Token()
				d := strings.TrimSpace(t.Data)
				if d == "" || noteRE.MatchString(d) {
					continue
				}
				data = append(data, d)
			}
		}
	}

	return data, nil
}

func href(href string) (*url.URL, error) {
	uri := &url.URL{}
	*uri = *burl

	p, err := url.Parse(href)
	if err != nil {
		return nil, err
	}

	if len(href) == 0 {
		return uri, nil
	} else if absURLRE.MatchString(href) || strings.HasPrefix(href, "//") {
		uri = p
		if uri.Host != burl.Host {
			return uri, nil
		}
		if uri.Scheme == "" {
			uri.Scheme = burl.Scheme
		}
		return uri, nil
	} else if href[0] == '/' {
		uri.Path = p.Path
		uri.RawQuery = p.RawQuery
		return uri, nil
	}

	uri.Path = path.Join(uri.Path, p.Path)
	uri.RawQuery = p.RawQuery

	return uri, nil
}
