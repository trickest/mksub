package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	roundChan "mksub/round"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	bufferSizeMB      = 100
	maxWorkingThreads = 100000
)

var (
	inputDomains []string
	wordSet      map[string]bool
	words        []string
)

var (
	domain       string
	domainFile   string
	wordlist     string
	regex        string
	nf           int
	level        int
	workers      int
	outputFolder string
	silent       bool

	workerThreadMax = make(chan struct{}, maxWorkingThreads)
	done            = make(chan struct{})
	wg              sync.WaitGroup
	wgWrite         sync.WaitGroup
	robin           roundChan.RoundRobin
)

func readDomainFile() {
	inputFile, err := os.Open(domainFile)
	if err != nil {
		panic("Could not open file to read domains!")
	}
	defer inputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	for scanner.Scan() {
		inputDomains = append(inputDomains, strings.TrimSpace(scanner.Text()))
	}
}

func prepareDomains() {
	if domain == "" && domainFile == "" {
		fmt.Println("No domain input provided")
		os.Exit(1)
	}

	inputDomains = make([]string, 0)
	if domain != "" {
		inputDomains = append(inputDomains, domain)
	} else {
		if domainFile != "" {
			readDomainFile()
		}
	}
}

func readWordlistFile() {
	var reg *regexp.Regexp
	var err error
	if regex != "" {
		reg, err = regexp.Compile(regex)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	wordlistFile, err := os.Open(wordlist)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer wordlistFile.Close()

	wordSet = make(map[string]bool)
	scanner := bufio.NewScanner(wordlistFile)
	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		word = strings.Trim(word, ".")
		if reg != nil {
			if !reg.Match([]byte(word)) {
				continue
			}
		}

		if word != "" {
			wordSet[word] = true
		}
	}

	for w := range wordSet {
		words = append(words, w)
	}
}

func closeWriters(number int) {
	for i := 0; i < number; i++ {
		done <- struct{}{}
	}
}

func spawnWriters(number int) {
	for i := 0; i < number; i++ {
		var bf bytes.Buffer
		ch := make(chan string, 100000)

		fileName := outputFolder
		if number > 1 {
			fileName += "-" + strconv.Itoa(i)
		}
		file, err := os.Create(path.Join(outputFolder, fileName))
		if err != nil {
			fmt.Println(err)
			fmt.Println("Couldn't open file to write output!")
		}

		wgWrite.Add(1)
		go write(file, &bf, &ch)

		if robin == nil {
			robin = roundChan.New(&ch)
			continue
		}
		robin.Add(&ch)
	}
}

func write(file *os.File, buffer *bytes.Buffer, ch *chan string) {
mainLoop:
	for {
		select {
		case <-done:
			for {
				if !writeOut(file, buffer, ch) {
					break
				}
			}
			if buffer.Len() > 0 {
				if file != nil {
					_, _ = file.WriteString(buffer.String())
					buffer.Reset()
				}
			}
			break mainLoop
		default:
			writeOut(file, buffer, ch)
		}
	}
	wgWrite.Done()
}
func writeOut(file *os.File, buffer *bytes.Buffer, outputChannel *chan string) bool {
	select {
	case s := <-*outputChannel:
		buffer.WriteString(s)
		if buffer.Len() >= bufferSizeMB*1024*1024 {
			_, _ = file.WriteString(buffer.String())
			buffer.Reset()
		}
		return true
	default:
		return false
	}
}

func combo(_comb string, level int, wg *sync.WaitGroup, wt *chan struct{}) {
	defer wg.Done()
	workerThreadMax <- struct{}{}

	if strings.Count(_comb, ".") > 1 {
		if !silent {
			fmt.Print(_comb + "\n")
		}
		*robin.Next() <- _comb + "\n"
	}

	var nextLevelWaitGroup sync.WaitGroup
	if level > 1 {
		nextLevelWt := make(chan struct{}, workers)
		for _, c := range words {
			nextLevelWaitGroup.Add(1)
			nextLevelWt <- struct{}{}
			go combo(c+"."+_comb, level-1, &nextLevelWaitGroup, &nextLevelWt)
		}
	} else {
		for _, c := range words {
			if !silent {
				fmt.Print(c + "." + _comb + "\n")
			}
			*robin.Next() <- c + "." + _comb + "\n"
		}
	}

	nextLevelWaitGroup.Wait()
	<-workerThreadMax
	<-*wt
}

func main() {
	flag.StringVar(&domain, "d", "", "Input domain")
	flag.StringVar(&domainFile, "df", "", "Input domain file, one domain per line")
	flag.StringVar(&wordlist, "w", "", "Wordlist file")
	flag.StringVar(&regex, "r", "", "Regex to filter words from wordlist file")
	flag.IntVar(&level, "l", 1, "Subdomain level to generate")
	flag.StringVar(&outputFolder, "o", "mksub-out", "Output folder (file(s) will use the same name)")
	flag.IntVar(&workers, "t", 100, "Number of threads for every subdomain level")
	flag.IntVar(&nf, "nf", 1, "Number of files to split the output into (faster with multiple files)")
	flag.BoolVar(&silent, "silent", true, "Skip writing generated subdomains to stdout (faster)")
	flag.Parse()

	go func() {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
		<-signalChannel

		fmt.Println("Program interrupted, exiting...")
		os.Exit(0)
	}()

	if level <= 0 || workers <= 0 {
		fmt.Println("Subdomain level and number of threads must be positive integers!")
		os.Exit(0)
	}

	dirInfo, err := os.Stat(outputFolder)
	dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

	if !dirExists {
		err = os.Mkdir(outputFolder, 0755)
		if err != nil {
			fmt.Println(err)
			fmt.Println("Couldn't create a directory to store outputs!")
			os.Exit(0)
		}
	}

	prepareDomains()
	readWordlistFile()
	spawnWriters(nf)

	for _, d := range inputDomains {
		wg.Add(1)
		wt := make(chan struct{}, 1)
		wt <- struct{}{}
		go combo(d, level, &wg, &wt)
	}

	wg.Wait()
	closeWriters(nf)
	wgWrite.Wait()
}
