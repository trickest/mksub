package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	domain := flag.String("d", "", "Input domain")
	domainFile := flag.String("df", "", "Input domain file, one domain per line")
	wordlist := flag.String("w", "", "Wordlist file")
	r := flag.String("r", "", "Regex to filter words from wordlist file")
	level := flag.Int("l", 1, "Subdomain level to generate (default 1)")
	output := flag.String("o", "", "Output file (optional)")
	flag.Parse()

	inputDomains := make([]string, 0)
	if *domain != "" {
		inputDomains = append(inputDomains, *domain)
	}
	if *domainFile != "" {
		inputFile, err := os.Open(*domainFile)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		defer inputFile.Close()
		scanner := bufio.NewScanner(inputFile)
		for scanner.Scan() {
			inputDomains = append(inputDomains, scanner.Text())
		}
	}
	if len(inputDomains) == 0 {
		fmt.Println("No input provided")
		os.Exit(1)
	}

	wordlistFile, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer wordlistFile.Close()

	var reg *regexp.Regexp
	if *r != "" {
		reg, err = regexp.Compile(*r)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	var outputFile *os.File
	if *output != "" {
		outputFile, err = os.Create(*output)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		defer outputFile.Close()
	}

	wordSet := make(map[string]bool)
	scanner := bufio.NewScanner(wordlistFile)

	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		word = strings.Trim(word, ".")
		if reg != nil {
			if !reg.Match([]byte(word)) {
				continue
			}
		}
		if _, isOld := wordSet[word]; word != "" && !isOld {
			wordSet[word] = true
		}
	}

	results := make([]string, 0)
	for i := 0; i < *level; i += 1 {
		toMerge := results[0:]
		if len(toMerge) == 0 {
			for word := range wordSet {
				results = append(results, word)
			}
		} else {
			for _, sd := range toMerge {
				for word := range wordSet {
					results = append(results, fmt.Sprintf("%s.%s", word, sd))
				}
			}
		}
	}
	for _, domain := range inputDomains {
		for _, subdomain := range results {
			fmt.Println(subdomain + "." + domain)
			if outputFile != nil {
				_, _ = outputFile.WriteString(subdomain + "." + domain + "\n")
			}
		}
	}
}
