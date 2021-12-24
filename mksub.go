package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

var (
	//flags
	domain     *string
	domainFile *string
	wordlist   *string
	regex      *string
	level      *int
	output     *string
	threads    *int

	inputDomains        chan string
	wordSet             map[string]bool
	outputChannel       chan string
	maxConcurrencyLevel = 1000000
	threadSemaphore     chan bool
)

func fileReadDomain(fileName string) {
	inputFile, err := os.Open(fileName)
	if err != nil {
		panic("Could not open file to read domains!")
	}
	defer inputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	for scanner.Scan() {
		inputDomains <- strings.TrimSpace(scanner.Text())
	}

	close(inputDomains)
}

func prepareDomains() {
	if *domain == "" && *domainFile == "" {
		fmt.Println("No domain input provided")
		os.Exit(1)
	}

	inputDomains = make(chan string, maxConcurrencyLevel)
	if *domain != "" {
		inputDomains <- *domain
	} else {
		if *domainFile != "" {
			fileReadDomain(*domainFile)
		}
	}
}

func processWordList(domain string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		<-threadSemaphore
	}()

	results := make([]string, 0)
	for word := range wordSet {
		results = append(results, word)
		outputChannel <- word + "." + domain
	}

	for i := 0; i < *level-1; i++ {
		for j := 0; j < len(results)-j*len(wordSet); j++ {
			sd := results[j]
			for word := range wordSet {
				results = append(results, word+"."+sd)
				outputChannel <- word + "." + sd + "." + domain
			}
		}
	}
}

func writeOutput(wg *sync.WaitGroup) {
	defer wg.Done()

	var outputFile *os.File
	var err error
	if *output != "" {
		outputFile, err = os.Create(*output)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		defer outputFile.Close()
	}

	for data := range outputChannel {
		fmt.Println(data)
		if outputFile != nil {
			_, _ = outputFile.WriteString(data + "\n")
		}
	}

}

func main() {
	domain = flag.String("d", "", "Input domain")
	domainFile = flag.String("df", "", "Input domain file, one domain per line")
	wordlist = flag.String("w", "", "Wordlist file")
	regex = flag.String("r", "", "Regex to filter words from wordlist file")
	level = flag.Int("l", 1, "Subdomain level to generate")
	output = flag.String("o", "", "Output file (optional)")
	threads = flag.Int("t", 100, "Number of threads to be used (maximum 1000000)")
	flag.Parse()

	go func() {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
		<-signalChannel

		fmt.Println("Program interrupted, exiting...")
		os.Exit(0)
	}()

	if *level <= 0 || *threads <= 0 {
		fmt.Println("Subdomain level and number of threads must be positive integers!")
		os.Exit(1)
	}

	if *threads > maxConcurrencyLevel {
		fmt.Println("Number of threads greater than the maximum number allowed (1000000)!")
		os.Exit(1)
	}

	go prepareDomains()

	var reg *regexp.Regexp
	var err error
	if *regex != "" {
		reg, err = regexp.Compile(*regex)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	wordlistFile, err := os.Open(*wordlist)
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

	outputChannel = make(chan string, *threads*maxConcurrencyLevel)

	var outWg sync.WaitGroup
	var inWg sync.WaitGroup

	outWg.Add(1)
	go writeOutput(&outWg)

	threadSemaphore = make(chan bool, maxConcurrencyLevel)

	for dom := range inputDomains {
		inWg.Add(1)
		threadSemaphore <- true
		go processWordList(dom, &inWg)
	}

	inWg.Wait()
	close(outputChannel)
	outWg.Wait()
}
