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

	inputDomains        chan string
	wordSet             map[string]bool
	outputChannel       chan string
	maxConcurrencyLevel = 1000000
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

	results := make([]string, 0)
	for word := range wordSet {
		results = append(results, word)
		outputChannel <- word + "." + domain
	}
	toMerge := results[0:]

	for i := 0; i < *level-1; i++ {
		toMerge = results[0:]
		for _, sd := range toMerge {
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

func factorial(n int) uint64 {
	var factVal uint64 = 1

	if n < 0 {
		fmt.Print("Factorial of a negative number doesn't exist.")
	} else {
		for i := 1; i <= n; i++ {
			factVal *= uint64(i)
		}
	}
	return factVal
}

func main() {
	domain = flag.String("d", "", "Input domain")
	domainFile = flag.String("df", "", "Input domain file, one domain per line")
	wordlist = flag.String("w", "", "Wordlist file")
	regex = flag.String("r", "", "Regex to filter words from wordlist file")
	level = flag.Int("l", 1, "Subdomain level to generate")
	output = flag.String("o", "", "Output file (optional)")
	flag.Parse()

	go func() {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
		<-signalChannel

		fmt.Println("Program interrupted, exiting...")
		os.Exit(0)
	}()

	if *level <= 0 {
		fmt.Println("Level must be a positive integer!")
		os.Exit(1)
	}
	prepareDomains()

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

	outputChannel = make(chan string, maxConcurrencyLevel)

	var outWg sync.WaitGroup
	var inWg sync.WaitGroup

	outWg.Add(1)
	go writeOutput(&outWg)

	for dom := range inputDomains {
		inWg.Add(1)
		go processWordList(dom, &inWg)
	}

	inWg.Wait()
	close(outputChannel)
	outWg.Wait()
}
