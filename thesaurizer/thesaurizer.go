package thesaurizer

import (
	"encoding/json"
	"flag"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
)

const (
	apiURL       = "https://api.api-ninjas.com/v1/thesaurus?word={}"
	punctuations = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	flag.Parse()
}

var apiKey string

func init() {
	// Get the bot token from the environment variable
	apiKey = os.Getenv("API_TOKEN")
	if apiKey == "" {
		log.Fatalf("API_TOKEN environment variable is not set")
	}
}

type ThesaurusResponse struct {
	Synonyms []string `json:"synonyms"`
}

// GetSynonyms replaces words in a phrase with their synonyms.
func GetSynonyms(phrase string) string {
	words := splitPhrase(phrase)

	var wg sync.WaitGroup
	resultChan := make(chan struct {
		index int
		word  string
	}, len(words))

	for i, word := range words {
		wg.Add(1)
		go func(index int, w string) {
			defer wg.Done()
			if isPunctuation(w) || w == "'" {
				resultChan <- struct {
					index int
					word  string
				}{index, w}
				return
			}
			synonym, err := getSynonym(w)
			if err != nil {
				log.Printf("Error fetching synonym for %s: %v\n", w, err)
				resultChan <- struct {
					index int
					word  string
				}{index, w}
				return
			}
			resultChan <- struct {
				index int
				word  string
			}{index, synonym}
		}(i, word)
	}

	wg.Wait()
	close(resultChan)

	// Collect results in order
	results := make([]string, len(words))
	for result := range resultChan {
		results[result.index] = result.word
	}

	// Build the final string with proper spacing
	var finalString strings.Builder
	flag := true // Indicates whether to add a space before the next word
	for _, word := range results {
		if word == "'" {
			finalString.WriteString("'")
			flag = true
			continue
		}
		if isPunctuation(word) {
			finalString.WriteString(word)
			continue
		}
		if !flag {
			finalString.WriteString(" ")
		}
		finalString.WriteString(word)
		flag = false
	}

	return finalString.String()
}

func splitPhrase(phrase string) []string {
	re := regexp.MustCompile(`\w+|[^\s\w]+`)
	return re.FindAllString(phrase, -1)
}

func isPunctuation(word string) bool {
	return strings.ContainsAny(word, punctuations)
}

func getSynonym(word string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", strings.Replace(apiURL, "{}", word, 1), nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var thesaurusResponse ThesaurusResponse
	if err := json.Unmarshal(body, &thesaurusResponse); err != nil {
		return "", err
	}

	if len(thesaurusResponse.Synonyms) > 0 {
		return thesaurusResponse.Synonyms[0], nil
	}
	return word, nil
}
