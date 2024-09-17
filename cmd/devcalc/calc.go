package main

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/frizinak/devcalc/dev"
	"github.com/frizinak/devcalc/devchart"
	"github.com/frizinak/devcalc/flags"
)

type Alias struct {
	Alias string
	Dev   string
	Dens  [2]float64
}

func (a Alias) Density() float64 {
	if a.Dens[1] == 0 {
		return 0
	}
	return a.Dens[0] / a.Dens[1]
}

type Format uint8

const (
	Format135 Format = 1 << iota
	Format120
	FormatSheet
)

var (
	cacheDir  string
	configDir string
	options   *devchart.Options
	stripMap  map[string]string
)

func strip(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, " ", "")
	f = strings.ReplaceAll(f, "-", "")
	f = strings.TrimRight(f, "%")
	return f
}

func unstrip(k string) (string, bool) {
	_, _ = getOptions()
	v, ok := stripMap[k]
	if !ok {
		return k, ok
	}
	return v, ok
}

func ex(err error) {
	if err == nil {
		return
	}
	fmt.Println(err)
	os.Exit(1)
}

func getCacheDir(subs ...string) string {
	if cacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			userDir, err := os.UserHomeDir()
			ex(err)
			userCacheDir = filepath.Join(userDir, ".cache")
		}
		cacheDir = filepath.Join(userCacheDir, "devcalc")
	}

	j := make([]string, 1+len(subs))
	copy(j[1:], subs)
	j[0] = cacheDir
	return filepath.Join(j...)
}

func getConfigDir(subs ...string) string {
	if configDir == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			userDir, err := os.UserHomeDir()
			ex(err)
			userConfigDir = filepath.Join(userDir, ".config")
		}
		configDir = filepath.Join(userConfigDir, "devcalc")
	}

	j := make([]string, 1+len(subs))
	copy(j[1:], subs)
	j[0] = configDir
	return filepath.Join(j...)
}

func aliasPath() string {
	dir := getConfigDir()
	_ = os.MkdirAll(dir, 0755)
	p := filepath.Join(dir, "aliases")
	_, err := os.Stat(p)
	if err != nil {
		f, err := os.Create(p)
		ex(err)
		f.Close()
	}

	return p
}

func tmpFile(file string) string {
	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	rnd := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, rnd)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf(
		"%s.%s-%s.tmp",
		file,
		stamp,
		base64.RawURLEncoding.EncodeToString(rnd),
	)
}

func parseDensity(div string) ([2]float64, error) {
	divs := strings.SplitN(div, "/", 2)
	var err error
	var dens [2]float64
	dens[0], err = strconv.ParseFloat(divs[0], 64)
	if err != nil {
		return dens, fmt.Errorf("invalid decimal number: '%s': %w", div, err)
	}
	dens[1], err = strconv.ParseFloat(divs[1], 64)
	if err != nil {
		return dens, fmt.Errorf("invalid decimal number: '%s': %w", div, err)
	}

	return dens, nil
}

func setAliases(aliases []Alias) error {
	clean := make([]Alias, 0, len(aliases))
	uniq := make(map[string]struct{}, len(aliases))
	for i := len(aliases) - 1; i >= 0; i-- {
		a := aliases[i].Alias
		if _, ok := uniq[a]; ok {
			continue
		}
		uniq[a] = struct{}{}
		clean = append(clean, aliases[i])
	}

	path := aliasPath()
	tmp := tmpFile(path)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	for i := len(clean) - 1; i >= 0; i-- {
		a := clean[i]
		f1 := strconv.FormatFloat(a.Dens[0], 'f', -1, 64)
		f2 := strconv.FormatFloat(a.Dens[1], 'f', -1, 64)
		_, err := fmt.Fprintf(f, "%s %s %s/%s\n", a.Alias, a.Dev, f1, f2)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}

func getAliases() ([]Alias, error) {
	l := make([]Alias, 0)
	uniq := make(map[string]struct{})
	p := aliasPath()
	f, err := os.Open(p)
	if err != nil {
		return l, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		text := scan.Text()
		f := strings.Fields(strings.TrimSpace(text))
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 {
			return l, fmt.Errorf("invalid line '%s'", text)
		}

		alias := f[0]
		dev := f[1]
		div := f[2]

		if _, ok := uniq[alias]; ok {
			return l, fmt.Errorf("duplicate alias '%s'", alias)
		}
		uniq[alias] = struct{}{}

		dens, err := parseDensity(div)
		if err != nil {
			return l, err
		}

		l = append(l, Alias{Alias: alias, Dev: dev, Dens: dens})
	}
	return l, scan.Err()
}

func getOptions() (devchart.Options, error) {
	if options == nil {
		o, err := devchart.GetOptions(getCacheDir("mdc"))
		if err != nil {
			return o, err
		}
		options = &o

		stripMap = make(map[string]string)
		for _, k := range o.Developers {
			stripMap[strip(k)] = k
		}
		for _, k := range o.Stocks {
			stripMap[strip(k)] = k
		}
	}
	return *options, nil
}

func printOptions(strs []string) {
	for _, e := range strs {
		fmt.Println(strip(e))
	}
}

func filterEntries(chem, stock, iso, ratio string) ([]devchart.Entry, error) {
	entries, err := devchart.Get(getCacheDir("mdc"), chem)
	if err != nil {
		return nil, err
	}

	stockQuery := wcGen(stock)
	filtered := make([]devchart.Entry, 0, len(entries))
	for _, e := range entries {
		if iso != "" && e.ISO != iso {
			continue
		}

		if ratio != "" {
			eratio := dev.ScaleString(dev.ScaleParts(e.Dilution))
			if eratio != ratio {
				continue
			}
		}

		name := strip(e.Name)
		if stock != "" && !wcMatch(stockQuery, name) {
			continue
		}

		filtered = append(filtered, e)
	}

	return filtered, nil
}

func printEntries(entries []devchart.Entry, format Format) {
	slices.SortFunc(entries, func(a, b devchart.Entry) int {
		if n := cmp.Compare(a.Developer, b.Developer); n != 0 {
			return n
		}

		if n := cmp.Compare(a.Name, b.Name); n != 0 {
			return n
		}

		isoA, err := strconv.Atoi(a.ISO)
		if err != nil {
			return 1
		}
		isoB, err := strconv.Atoi(b.ISO)
		if err != nil {
			return -1
		}

		if n := cmp.Compare(isoA, isoB); n != 0 {
			return n
		}

		if n := cmp.Compare(a.Dilution, b.Dilution); n != 0 {
			return n
		}

		if n := cmp.Compare(a.Temp, b.Temp); n != 0 {
			return n
		}

		return 0
	})

	r := func(dur time.Duration) string {
		s := int(dur.Seconds())
		buf := bytes.NewBuffer(make([]byte, 8))
		//return fmt.Sprintf("%dh
		if s >= 3600 {
			h := s / 3600
			s -= h * 3600

			buf.WriteString(strconv.Itoa(h))
			buf.WriteString("h")
		}

		if s >= 60 {
			m := s / 60
			s -= m * 60
			buf.WriteString(strconv.Itoa(m))
			buf.WriteString("m")
		}

		if s != 0 {
			buf.WriteString(strconv.Itoa(s))
			buf.WriteString("s")
		}

		return buf.String()
	}

	for _, e := range entries {
		ratio := dev.ScaleString(dev.ScaleParts(e.Dilution))
		notes := make([]string, 1, 1+len(e.Notes))
		for _, n := range e.Notes {
			notes = append(notes, fmt.Sprintf("        %s", n))
		}

		if len(notes) == 1 {
			notes = notes[:0]
		}

		dur := make([]string, 0, 3)

		if format&Format135 != 0 && e.T135 != 0 {
			dur = append(dur, fmt.Sprintf("[135: %s]", r(e.T135)))
		}
		if format&Format120 != 0 && e.T120 != 0 {
			dur = append(dur, fmt.Sprintf("[120: %s]", r(e.T120)))
		}
		if format&FormatSheet != 0 && e.TSheet != 0 {
			dur = append(dur, fmt.Sprintf("[sheet: %s]", r(e.TSheet)))
		}

		fmt.Printf(
			"%6s) %s %s %.1fC %s%s\n",
			e.ISO,
			strip(e.Name),
			ratio,
			e.Temp,
			strings.Join(dur, " "),
			strings.Join(notes, "\n"),
		)
	}
}

func wcGen(str string) []string {
	return strings.Split(strings.ToLower(str), "*")
}

func wcMatch(query []string, target string) bool {
	lc := strings.ToLower(target)
	for i, p := range query {
		method := strings.Contains
		if i == 0 {
			method = strings.HasPrefix
		}
		if i == len(query)-1 {
			method = strings.HasSuffix
		}

		if p == "" {
			continue
		}

		if len(query) == 1 {
			return lc == p
		}

		if !method(lc, p) {
			return false
		}
	}
	return true
}

type Size struct{ X, Y int }

func termSize() Size {
	termsize, err := console.Current().Size()
	if err != nil {
		return Size{}
	}

	return Size{int(termsize.Width), int(termsize.Height)}
}

func main() {
	fr := flags.NewRoot(os.Stdout)

	fr.Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, "  ", set.Name(), "calc:  Calculate developing volumes")
			fmt.Fprintln(w, "  ", set.Name(), "alias: Alias developers and optionally store densities")
			fmt.Fprintln(w, "  ", set.Name(), "mdc:   Massive Dev Chart operations")
			fmt.Fprintln(w, "  ", set.Name(), "timer: Run a developing timer")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		set.Usage(1)
		return nil
	})

	fr.Add("timer").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Run developing timer")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, "  ", set.Name(), "<total> <initial> <agitation> <interval> [delay]")
			fmt.Fprintln(w, "  <total>     required, total development duration (e.g.: 7m30s)")
			fmt.Fprintln(w, "  <initial>   required, duration of initial agitation phase (e.g.: 0:30)")
			fmt.Fprintln(w, "  <agitation> required, duration of normal agitation phases (e.g.: 10s)")
			fmt.Fprintln(w, "  <interval>  required, interval of normal agitation phases (e.g.: 30)")
			fmt.Fprintln(w, "  [delay]     optional, initial delay (e.g.: 5)")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		if len(args) < 4 || len(args) > 5 {
			set.Usage(1)
		}

		durs := make([]time.Duration, 5)
		for i, d := range args {
			if len(d) == 0 {
				set.Usage(1)
			}

			last := d[len(d)-1]
			if last != 's' && last != 'm' {
				d = d + "s"
			}
			d = strings.Replace(d, ":", "m", 1)

			var err error
			durs[i], err = time.ParseDuration(d)
			if err != nil {
				return fmt.Errorf("could not parse '%s'", args[i])
			}
		}

		durTotal := durs[0]
		durInit := durs[1]
		durAgi := durs[2]
		durIv := durs[3]
		delay := durs[4]

		if durAgi == 0 {
			durIv = 0
		} else if durIv <= durAgi {
			return fmt.Errorf("can not have an agitation interval that is lower than or equal to the agitation duration")
		}

		s := time.Now()
		phase := 0
		if delay == 0 {
			phase = 1
		}

		var pdur func(dur time.Duration) string
		{
			const remDiv = 100 // determines framerate
			prec := math.Log10(1000 / remDiv)
			format := fmt.Sprintf("%%02d:%%02d.%%0%dd", int(prec))
			pdur = func(dur time.Duration) string {
				huns := dur.Milliseconds() / remDiv

				const md = 60000 / remDiv
				mins := huns / md
				huns -= mins * md

				const sd = 1000 / remDiv
				secs := huns / sd
				huns -= secs * sd

				return fmt.Sprintf(format, mins, secs, huns)
			}
		}

		buf := bytes.NewBuffer(make([]byte, 0, 4096))

		var clear = []byte("\033[2J\033[H")
		var clrReset = []byte("\033[0m")

		var clrFGLight = []byte("\033[1m\033[38;5;255m")
		var clrFGRed = []byte("\033[48;5;52m\033[38;5;255m")
		var clrFGGreen = []byte("\033[48;5;22m\033[38;5;255m")

		var clrBGRed = []byte("\033[48;5;88m")
		var clrBGGreen = []byte("\033[48;5;34m")
		var clrBGBlack = []byte("\033[48;5;233m")

		const space = ' '
		const nl = '\n'
		var lastTermMeasure time.Time
		var term Size

		out := func(clrFG, clrBG []byte, str string) {
			// len should be ok as long as we don't use any unicode that
			// could be printed wider.
			w := (term.X - len(str) - 2) / 2
			for i := 0; i < w; i++ {
				buf.WriteByte(space)
			}
			buf.Write(clrFG)
			buf.WriteByte(space)
			buf.WriteString(str)
			buf.WriteByte(space)
			buf.Write(clrBG)
			for i := 0; i < term.X-w-len(str)-2; i++ {
				buf.WriteByte(space)
			}
			buf.WriteByte(nl)
		}

		type c struct{ fg, bg, high []byte }
		var clr c
		agis := time.Duration(0)

		type o struct {
			label, remaining, total string
		}
		var output, lastOutput o
		var done, exit, force bool

		for {
			since := time.Since(s) - delay
			left := durTotal - since

			switch {
			case left <= -time.Second*10:
				exit = true
			case left <= 0:
				if (int(since.Seconds()*5)%2 == 0) == done {
					done = !done
					force = true
				}
				clr = c{clrFGLight, clrBGRed, clrFGRed}
				output = o{"", "Done", ""}
				if done {
					clr = c{clrFGLight, clrBGGreen, clrFGGreen}
				}
			case phase == 0:
				if since >= 0 {
					phase = 1
					continue
				}
				clr = c{clrFGLight, clrBGBlack, clrFGLight}
				output = o{"wait", pdur(-since), ""}
			case phase == 1:
				rem := durInit - since
				if rem > left {
					rem = left
				}
				if rem <= 0 {
					phase = 2
					continue
				}
				clr = c{clrFGLight, clrBGRed, clrFGRed}
				output = o{"AGITATE!", pdur(rem), pdur(left)}
			case phase == 2:
				rem := (agis+1)*durIv - (since - durInit)
				if durIv <= 0 {
					rem = left
				}
				if rem > left {
					rem = left
				}
				if rem <= 0 {
					agis++
					phase = 3
					continue
				}
				clr = c{clrFGLight, clrBGGreen, clrFGGreen}
				output = o{"Developing", pdur(rem), pdur(left)}
			case phase == 3:
				rem := agis*durIv + durAgi - (since - durInit)
				if rem > left {
					rem = left
				}
				if rem <= 0 {
					phase = 2
					continue
				}

				clr = c{clrFGLight, clrBGRed, clrFGRed}
				output = o{"AGITATE!", pdur(rem), pdur(left)}
			}

			if time.Since(lastTermMeasure) > time.Millisecond*100 {
				lastTermMeasure = time.Now()
				term = termSize()
			}

			if output != lastOutput || force {
				force = false
				lastOutput = output
				buf.Write(clear)
				lw := 5
				var i int
				for ; i < (term.Y-lw)/2+1; i++ {
					buf.Write(clr.bg)
					for i := 0; i < term.X; i++ {
						buf.WriteByte(space)
					}
					buf.WriteByte(nl)
				}
				lw += i

				out(clr.fg, clr.bg, output.label)
				out(clr.fg, clr.bg, "")
				out(clr.high, clr.bg, output.remaining)
				out(clr.fg, clr.bg, "")
				out(clr.fg, clr.bg, output.total)

				for i := lw; i < term.Y; i++ {
					buf.Write(clr.bg)
					for i := 0; i < term.X; i++ {
						buf.WriteByte(space)
					}
					buf.WriteByte(nl)
				}

				buf.Write(clrReset)

				buf.WriteTo(os.Stdout)
				buf.Reset()
			}

			if exit {
				break
			}

			time.Sleep(time.Millisecond * 10)
		}

		return nil
	})

	var cmdMDC *flags.Set
	cmdMDC = fr.Add("mdc").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Massive dev chart commands")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, "  ", set.Name(), "list:   Get a listing of developers or stocks")
			fmt.Fprintln(w, "  ", set.Name(), "get     Get the Massive Dev Chart table (with notes) of a specific developer")
			fmt.Fprintln(w, "  ", set.Name(), "getall: Get all Massive Dev Chart tables, effectively caching all data")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		cmdMDC.Usage(1)
		return nil
	})

	var cmdMDCList *flags.Set
	cmdMDCList = cmdMDC.Add("list").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "List Massive Dev Chart data")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, "  ", set.Name(), "developers")
			fmt.Fprintln(w, "  ", set.Name(), "stocks")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		cmdMDCList.Usage(1)
		return nil
	})

	cmdMDCList.Add("developers").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "List all Massive Dev Chart developers")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		o, err := getOptions()
		if err != nil {
			return err
		}
		printOptions(o.Developers)
		return nil
	})

	cmdMDCList.Add("stocks").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "List all Massive Dev Chart film stocks")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		o, err := getOptions()
		if err != nil {
			return err
		}
		printOptions(o.Stocks)
		return nil
	})

	cmdMDCGet := cmdMDC.Add("get").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Get development times for the given developer and stock")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, set.Name(), "<developer>", "[stock]", "[iso]")
			fmt.Fprintln(w, "  <developer>  required, use `mdc list developers` to get a listing.")
			fmt.Fprintln(w, "  [stock]      optional, use `mdc list stocks`     to get a listing. supports * for wildcard matching.")
			fmt.Fprintln(w, "  [iso]        optional, only show entries with the specified iso.")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		if len(args) == 0 || len(args) > 3 {
			set.Usage(1)
		}

		chem, _ := unstrip(args[0])
		var stock, iso string
		if len(args) > 1 {
			stock = strip(args[1])
		}
		if len(args) > 2 {
			iso = args[2]
		}

		filtered, err := filterEntries(chem, stock, iso, "")
		if err != nil {
			return err
		}

		printEntries(filtered, Format135|Format120|FormatSheet)

		return nil
	})

	cmdMDC.Add("getall").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Get all Massive Dev Chart tables, effectively caching all data")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		o, err := getOptions()
		if err != nil {
			return err
		}
		for _, dev := range o.Developers {
			entries, err := devchart.Get(getCacheDir("mdc"), dev)
			if errors.As(err, &devchart.NotExistsError{}) {
				continue
			}
			if err != nil {
				return err
			}
			fmt.Println(strip(dev))
			printEntries(entries, Format135|Format120|FormatSheet)
			fmt.Println("")
		}
		return nil
	})

	fr.Add("alias").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Alias a developer to a different name and optionally store its density")
			fmt.Fprintln(w, "Aliases are stored in ", aliasPath()) // can cause an exit
			fmt.Fprintln(w, "(e.g. alias adox.adonal rodinal 280/200 to set the density of adox.adonal to 1.4g/ml and use it as a rodinal alias")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, set.Name(), "<alias>", "<developer>", "[density]")
			fmt.Fprintln(w, "  <alias>      required")
			fmt.Fprintln(w, "  <developer>  required, use `mdc list developers` to get a listing")
			fmt.Fprintln(w, "  [density]    optional, the density, can be a decimal number or a fraction (e.g.: 0.7 or 300.5/1000)")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		if len(args) < 2 || len(args) > 3 {
			set.Usage(1)
			return nil
		}

		if _, ok := unstrip(args[1]); !ok {
			return fmt.Errorf("no such developer: '%s'", args[1])
		}

		var dens [2]float64
		if len(args) == 3 {
			var err error
			dens, err = parseDensity(args[2])
			if err != nil {
				return err
			}
		}

		aliases, err := getAliases()
		ex(err)

		aliases = append(aliases, Alias{args[0], args[1], dens})
		return setAliases(aliases)
	})

	fr.Add("calc").Define(func(set *flag.FlagSet) func(io.Writer) {
		return func(w io.Writer) {
			fmt.Fprintln(w, "Calculate developing volumes")
			fmt.Fprintln(w, "Usage:")
			fmt.Fprintln(w, set.Name(), "<developer>", "<ratio>", "<volume>", "[stock]", "[iso]")
			fmt.Fprintln(w, "  <developer>  required, use `mdc list developers` to get a listing.")
			fmt.Fprintln(w, "                         can also be any of your aliases with a stored density for mixing by weight.")
			fmt.Fprintln(w, "  <ratio>      required, the dilution to use.")
			fmt.Fprintln(w, "  <volume>     required, the total developing volume (ml).")
			fmt.Fprintf(w, "  [stock]      optional, also print developing information. (see: %s)\n", cmdMDCGet.Name())
			fmt.Fprintln(w, "  [iso]        optional, only show entries with the specified iso.")
		}
	}).Handler(func(set *flags.Set, args []string) error {
		if len(args) < 3 || len(args) > 5 {
			set.Usage(1)
			return nil
		}

		aliases := make(map[string]Alias)
		{
			_a, err := getAliases()
			if err != nil {
				return err
			}
			for _, a := range _a {
				aliases[a.Alias] = a
			}
		}

		chem := args[0]
		ratio := args[1]
		vol, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			return fmt.Errorf("invalid volume")
		}

		var stock, iso string
		if len(args) > 3 {
			stock = args[3]
		}
		if len(args) > 4 {
			iso = args[4]
		}
		alias := aliases[chem]

		fmt.Println(dev.Calc(dev.NewChem(alias.Density(), dev.ScaleRatio(ratio)), vol))
		if stock == "" {
			return nil
		}

		if alias.Dev != "" {
			chem = alias.Dev
		}
		chem, _ = unstrip(chem)
		qratio := dev.ScaleString(dev.ScaleParts(ratio))

		filtered, err := filterEntries(chem, stock, iso, qratio)
		if err != nil {
			return err
		}
		printEntries(filtered, Format135|Format120|FormatSheet)

		return nil
	})

	f, _ := fr.ParseCommandline()
	ex(f.Do())
}
